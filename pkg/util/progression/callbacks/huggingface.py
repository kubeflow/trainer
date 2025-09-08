#!/usr/bin/env python3

"""
HuggingFace Transformers Trainer callback for TrainJob progression tracking.

This callback writes training progression status to a JSON file that can be
read by the TrainJob controller to track training progress.

Usage:
    from examples.pytorch.progression_callback import TrainJobProgressionCallback
"""

import json
import os
import time
from typing import Optional

from transformers import (
    TrainerCallback,
    TrainerControl,
    TrainerState,
    TrainingArguments,
)


class TrainJobProgressionCallback(TrainerCallback):
    """Callback to track training progression for TrainJob controller."""

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
        self.status_file_path = status_file_path or os.getenv(
            "TRAINJOB_PROGRESSION_FILE_PATH", "/tmp/training_progression.json"
        )
        self.update_interval = update_interval
        self.last_update_time = 0
        self.start_time = time.time()

    def on_train_begin(
        self,
        args: TrainingArguments,
        state: TrainerState,
        control: TrainerControl,
        **kwargs,
    ):
        """Called when training begins."""
        self.start_time = time.time()
        self._write_status(args, state, "Training started")

    def on_step_end(
        self,
        args: TrainingArguments,
        state: TrainerState,
        control: TrainerControl,
        **kwargs,
    ):
        """Called at the end of each training step."""
        current_time = time.time()
        if current_time - self.last_update_time >= self.update_interval:
            self._write_status(args, state, "Training in progress")
            self.last_update_time = current_time

    def on_epoch_end(
        self,
        args: TrainingArguments,
        state: TrainerState,
        control: TrainerControl,
        **kwargs,
    ):
        """Called at the end of each epoch."""
        self._write_status(args, state, f"Completed epoch {state.epoch}")

    def on_train_end(
        self,
        args: TrainingArguments,
        state: TrainerState,
        control: TrainerControl,
        **kwargs,
    ):
        """Called when training ends."""
        self._write_status(args, state, "Training completed")

    def _write_status(self, args: TrainingArguments, state: TrainerState, message: str):
        """Write current training status to file."""
        try:
            # Calculate total steps
            total_steps = (
                args.max_steps
                if args.max_steps > 0
                else (
                    len(state.train_dataloader) * args.num_train_epochs
                    if hasattr(state, "train_dataloader") and state.train_dataloader
                    else None
                )
            )

            # Separate structured training metrics and generic metrics
            training_metrics = {}
            generic_metrics = {}

            if state.log_history:
                latest_log = state.log_history[-1]
                for key, value in latest_log.items():
                    if isinstance(value, (int, float)):
                        str_value = str(value)

                        # Map to structured TrainingMetrics fields
                        if key in ["train_loss", "loss"]:
                            training_metrics["loss"] = str_value
                        elif key in ["learning_rate", "lr"]:
                            training_metrics["learning_rate"] = str_value
                        elif key in ["eval_accuracy", "accuracy", "train_accuracy"]:
                            training_metrics["accuracy"] = str_value
                        else:
                            # Everything else goes to generic metrics
                            generic_metrics[key] = str_value

            # Add checkpoint information
            if hasattr(state, "best_metric") and state.best_metric is not None:
                generic_metrics["best_metric"] = str(state.best_metric)

            # Add checkpoint count if available
            if hasattr(args, "output_dir") and args.output_dir:
                import glob

                checkpoint_pattern = f"{args.output_dir}/checkpoint-*"
                checkpoints = glob.glob(checkpoint_pattern)
                if checkpoints:
                    training_metrics["checkpoints_stored"] = len(checkpoints)

                    # Get latest checkpoint by highest checkpoint number
                    def get_checkpoint_number(checkpoint_path):
                        try:
                            return int(checkpoint_path.split("-")[-1])
                        except (IndexError, ValueError):
                            return -1

                    latest_checkpoint = max(checkpoints, key=get_checkpoint_number)
                    training_metrics["latest_checkpoint_path"] = latest_checkpoint

            # Prepare status data
            status_data = {
                "current_step": state.global_step,
                "total_steps": total_steps,
                "current_epoch": int(state.epoch) if state.epoch else None,
                "total_epochs": (
                    int(args.num_train_epochs) if args.num_train_epochs else None
                ),
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


# Example usage
if __name__ == "__main__":
    """
    Example of how to use the TrainJobProgressionCallback with HuggingFace Trainer.
    """
    from transformers import Trainer, TrainingArguments

    # Initialize training arguments
    training_args = TrainingArguments(
        output_dir="./results",
        num_train_epochs=3,
        per_device_train_batch_size=16,
        logging_steps=10,
        save_steps=500,
    )

    # Create the progression callback
    progression_callback = TrainJobProgressionCallback()

    # Initialize trainer with the callback
    trainer = Trainer(
        args=training_args,
        # Add your model, tokenizer, dataset here
        callbacks=[progression_callback],
    )

    # Start training
    # trainer.train()
