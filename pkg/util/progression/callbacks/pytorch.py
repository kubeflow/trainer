#!/usr/bin/env python3

"""
PyTorch training loop example with TrainJob progression tracking.

This example shows how to instrument a custom PyTorch training loop
to write progression status for the TrainJob controller.
"""

import json
import os
import time
from typing import Optional

import torch
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import DataLoader


class ProgressionTracker:
    """Helper class to track and write training progression."""

    def __init__(
        self,
        total_epochs: int,
        steps_per_epoch: int,
        status_file_path: Optional[str] = None,
        update_interval: int = 30,
    ):
        """
        Initialize progression tracker.

        Args:
            total_epochs: Total number of training epochs
            steps_per_epoch: Number of steps per epoch
            status_file_path: Path where progression status will be written.
                            If None, uses TRAINJOB_PROGRESSION_FILE_PATH env var or default.
            update_interval: Minimum seconds between status updates
        """
        self.total_epochs = total_epochs
        self.steps_per_epoch = steps_per_epoch
        self.total_steps = total_epochs * steps_per_epoch
        self.status_file_path = status_file_path or os.getenv(
            "TRAINJOB_PROGRESSION_FILE_PATH", "/tmp/training_progression.json"
        )
        self.update_interval = update_interval
        self.start_time = time.time()
        self.last_update_time = 0
        self.current_epoch = 0
        self.current_step = 0
        self.metrics = {}

    def update_step(
        self,
        epoch: int,
        step: int,
        loss: float = None,
        learning_rate: float = None,
        **kwargs,
    ):
        """Update current step and optionally write status."""
        self.current_epoch = epoch
        self.current_step = epoch * self.steps_per_epoch + step

        # Update metrics
        if loss is not None:
            self.metrics["loss"] = str(loss)
        if learning_rate is not None:
            self.metrics["learning_rate"] = str(learning_rate)

        # Add any additional metrics
        for key, value in kwargs.items():
            if isinstance(value, (int, float)):
                self.metrics[key] = str(value)

        # Write status if enough time has passed
        current_time = time.time()
        if current_time - self.last_update_time >= self.update_interval:
            message = (
                f"Training epoch {epoch+1}/{self.total_epochs}, "
                f"step {step+1}/{self.steps_per_epoch}"
            )
            self.write_status(message)
            self.last_update_time = current_time

    def update_epoch(self, epoch: int, **metrics):
        """Update current epoch and write status."""
        self.current_epoch = epoch

        # Update metrics
        for key, value in metrics.items():
            if isinstance(value, (int, float)):
                self.metrics[key] = str(value)

        self.write_status(f"Completed epoch {epoch+1}/{self.total_epochs}")

    def write_status(self, message: str = "Training in progress"):
        """Write current training status to file."""
        try:
            status_data = {
                "current_step": self.current_step,
                "total_steps": self.total_steps,
                "current_epoch": self.current_epoch,
                "total_epochs": self.total_epochs,
                "message": message,
                "metrics": self.metrics.copy(),
                "timestamp": int(time.time()),
                "start_time": int(self.start_time),
            }

            # Write to file atomically
            temp_file = f"{self.status_file_path}.tmp"
            with open(temp_file, "w") as f:
                json.dump(status_data, f, indent=2)
            os.rename(temp_file, self.status_file_path)

        except Exception as e:
            print(f"Failed to write progression status: {e}")


def train_model_with_progression(
    model, train_loader, criterion, optimizer, num_epochs: int, device: str = "cpu"
):
    """
    Example training loop with progression tracking.

    Args:
        model: PyTorch model to train
        train_loader: DataLoader for training data
        criterion: Loss function
        optimizer: Optimizer
        num_epochs: Number of training epochs
        device: Device to train on
    """

    # Initialize progression tracker
    steps_per_epoch = len(train_loader)
    tracker = ProgressionTracker(
        total_epochs=num_epochs, steps_per_epoch=steps_per_epoch
    )

    # Write initial status
    tracker.write_status("Training started")

    model.train()

    for epoch in range(num_epochs):
        epoch_loss = 0.0

        for step, (inputs, targets) in enumerate(train_loader):
            inputs, targets = inputs.to(device), targets.to(device)

            # Forward pass
            optimizer.zero_grad()
            outputs = model(inputs)
            loss = criterion(outputs, targets)

            # Backward pass
            loss.backward()
            optimizer.step()

            # Update progression
            current_lr = optimizer.param_groups[0]["lr"]
            tracker.update_step(
                epoch=epoch, step=step, loss=loss.item(), learning_rate=current_lr
            )

            epoch_loss += loss.item()

        # Update epoch progression
        avg_loss = epoch_loss / steps_per_epoch
        tracker.update_epoch(epoch, avg_loss=avg_loss)

        print(f"Epoch {epoch+1}/{num_epochs}, Average Loss: {avg_loss:.4f}")

    # Write final status
    tracker.write_status("Training completed")
    print("Training completed!")


# Example usage
if __name__ == "__main__":
    """
    Example of how to use progression tracking in a PyTorch training script.
    """

    # Create a simple model and data
    model = nn.Sequential(nn.Linear(10, 50), nn.ReLU(), nn.Linear(50, 1))

    # Create dummy data
    dummy_data = [(torch.randn(32, 10), torch.randn(32, 1)) for _ in range(100)]
    train_loader = DataLoader(dummy_data, batch_size=1, shuffle=True)

    criterion = nn.MSELoss()
    optimizer = optim.Adam(model.parameters(), lr=0.001)

    # Train with progression tracking
    train_model_with_progression(
        model=model,
        train_loader=train_loader,
        criterion=criterion,
        optimizer=optimizer,
        num_epochs=5,
    )
