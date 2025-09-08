#!/usr/bin/env python3

"""
TorchTune progression callback for TrainJob controller integration.

This callback can be used with TorchTune recipes to automatically track
training progression for the TrainJob controller.
"""

import json
import os
import time
from typing import Any, Dict, Optional

from torchtune.training import TrainingCallback


class TrainJobProgressionCallback(TrainingCallback):
    """TorchTune callback to track training progression for TrainJob controller."""

    def __init__(
        self, status_file_path: Optional[str] = None, update_interval: int = 30
    ):
        """
        Initialize the progression callback.

        Args:
            status_file_path: Path where progression status will be written.
                            If None, uses TRAINJOB_PROGRESSION_FILE_PATH env var or default.
            update_interval: Minimum seconds between status updates
        """
        super().__init__()
        self.status_file_path = status_file_path or os.getenv(
            "TRAINJOB_PROGRESSION_FILE_PATH", "/tmp/training_progression.json"
        )
        self.update_interval = update_interval
        self.last_update_time = 0
        self.start_time = time.time()
        self.total_epochs = None
        self.steps_per_epoch = None
        self.total_steps = None

    def on_train_start(self, state: Dict[str, Any]) -> None:
        """Called when training starts."""
        self.start_time = time.time()

        # Extract training configuration
        if "epochs" in state:
            self.total_epochs = state["epochs"]
        if "steps_per_epoch" in state:
            self.steps_per_epoch = state["steps_per_epoch"]
            if self.total_epochs:
                self.total_steps = self.total_epochs * self.steps_per_epoch
        elif "max_steps" in state:
            self.total_steps = state["max_steps"]

        self._write_status(state, "Training started")

    def on_step_end(self, state: Dict[str, Any]) -> None:
        """Called at the end of each training step."""
        current_time = time.time()
        if current_time - self.last_update_time >= self.update_interval:
            self._write_status(state, "Training in progress")
            self.last_update_time = current_time

    def on_epoch_end(self, state: Dict[str, Any]) -> None:
        """Called at the end of each epoch."""
        epoch = state.get("epoch", 0)
        self._write_status(state, f"Completed epoch {epoch}")

    def on_train_end(self, state: Dict[str, Any]) -> None:
        """Called when training ends."""
        self._write_status(state, "Training completed")

    def _write_status(self, state: Dict[str, Any], message: str):
        """Write current training status to file."""
        try:
            # Extract current progress
            current_step = state.get("step", 0)
            current_epoch = state.get("epoch", 0)

            # Separate structured training metrics and generic metrics
            training_metrics = {}
            generic_metrics = {}

            # Common metrics that might be in state
            metric_keys = [
                "loss",
                "lr",
                "learning_rate",
                "train_loss",
                "val_loss",
                "accuracy",
                "perplexity",
                "grad_norm",
            ]

            for key in metric_keys:
                if key in state and isinstance(state[key], (int, float)):
                    str_value = str(state[key])

                    # Map to structured TrainingMetrics fields
                    if key in ["loss", "train_loss"]:
                        training_metrics["loss"] = str_value
                    elif key in ["lr", "learning_rate"]:
                        training_metrics["learning_rate"] = str_value
                    elif key == "accuracy":
                        training_metrics["accuracy"] = str_value
                    else:
                        # Everything else goes to generic metrics
                        generic_metrics[key] = str_value

            # Add checkpoint information if available
            if "checkpoint_path" in state:
                training_metrics["latest_checkpoint_path"] = str(
                    state["checkpoint_path"]
                )
            if "checkpoints_saved" in state:
                training_metrics["checkpoints_stored"] = (
                    int(state["checkpoints_saved"])
                    if isinstance(state["checkpoints_saved"], (int, float))
                    else 0
                )

            # Prepare status data
            status_data = {
                "current_step": current_step,
                "total_steps": self.total_steps,
                "current_epoch": current_epoch,
                "total_epochs": self.total_epochs,
                "message": message,
                "timestamp": int(time.time()),
                "start_time": int(self.start_time),
            }

            # Add structured training metrics if any
            if training_metrics:
                status_data["training_metrics"] = training_metrics

            # Add generic metrics if any
            if generic_metrics:
                status_data["metrics"] = generic_metrics

            # Write to file atomically
            temp_file = f"{self.status_file_path}.tmp"
            with open(temp_file, "w") as f:
                json.dump(status_data, f, indent=2)
            os.rename(temp_file, self.status_file_path)

        except Exception as e:
            print(f"Failed to write progression status: {e}")


# Example integration with TorchTune recipe
def add_progression_callback_to_recipe(recipe_config: Dict[str, Any]) -> Dict[str, Any]:
    """
    Helper function to add progression callback to a TorchTune recipe configuration.

    Args:
        recipe_config: TorchTune recipe configuration dictionary

    Returns:
        Modified recipe configuration with progression callback
    """
    if "callbacks" not in recipe_config:
        recipe_config["callbacks"] = []

    # Add progression callback
    progression_callback_config = {
        "_component_": "cmd.trainers.torchtune.progression_callback.TrainJobProgressionCallback",
        "status_file_path": "/tmp/training_progression.json",
        "update_interval": 30,
    }

    recipe_config["callbacks"].append(progression_callback_config)
    return recipe_config


if __name__ == "__main__":
    """
    Example of how to use the progression callback with TorchTune.
    """

    # Example recipe configuration modification
    sample_config = {
        "model": {"_component_": "torchtune.models.llama2.llama2_7b"},
        "tokenizer": {"_component_": "torchtune.models.llama2.llama2_tokenizer"},
        "optimizer": {"_component_": "torch.optim.AdamW", "lr": 2e-5},
        "epochs": 3,
        "max_steps_per_epoch": 1000,
    }

    # Add progression callback
    config_with_progression = add_progression_callback_to_recipe(sample_config)

    print("Recipe configuration with progression callback:")
    print(json.dumps(config_with_progression, indent=2))
