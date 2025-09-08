#!/usr/bin/env python3

"""
Simple tests for progression callback structure and basic functionality.
"""

import json
import os
import tempfile
import unittest
from unittest.mock import patch

# Import our callbacks
from pytorch import ProgressionTracker


class TestProgressionCallbackStructure(unittest.TestCase):
    """Test basic callback structure and functionality."""

    def setUp(self):
        """Set up test fixtures."""
        self.temp_file = tempfile.mktemp(suffix=".json")

    def tearDown(self):
        """Clean up test fixtures."""
        if os.path.exists(self.temp_file):
            os.remove(self.temp_file)

    def test_pytorch_tracker_basic_functionality(self):
        """Test PyTorch tracker basic functionality."""
        tracker = ProgressionTracker(
            total_epochs=2,
            steps_per_epoch=10,
            status_file_path=self.temp_file,
            update_interval=0,  # Always update for testing
        )

        # Test basic properties
        self.assertEqual(tracker.total_epochs, 2)
        self.assertEqual(tracker.steps_per_epoch, 10)
        self.assertEqual(tracker.total_steps, 20)
        self.assertEqual(tracker.status_file_path, self.temp_file)

        # Test write_status
        tracker.write_status("Test message")

        # Verify file was created and contains valid JSON
        self.assertTrue(os.path.exists(self.temp_file))

        with open(self.temp_file, "r") as f:
            data = json.load(f)

        self.assertEqual(data["message"], "Test message")
        self.assertEqual(data["total_epochs"], 2)
        self.assertEqual(data["total_steps"], 20)
        self.assertIn("timestamp", data)

    def test_pytorch_tracker_with_env_var(self):
        """Test PyTorch tracker respects environment variable."""
        with patch.dict(os.environ, {"TRAINJOB_PROGRESSION_FILE_PATH": self.temp_file}):
            tracker = ProgressionTracker(total_epochs=1, steps_per_epoch=5)
            self.assertEqual(tracker.status_file_path, self.temp_file)

    def test_update_step_with_metrics(self):
        """Test step update with metrics."""
        tracker = ProgressionTracker(
            total_epochs=1,
            steps_per_epoch=10,
            status_file_path=self.temp_file,
            update_interval=0,
        )

        tracker.update_step(
            epoch=0,
            step=5,
            loss=0.5,
            learning_rate=0.001,
            accuracy=0.85,  # Numeric custom metric
        )

        with open(self.temp_file, "r") as f:
            data = json.load(f)

        self.assertEqual(data["current_step"], 5)
        self.assertEqual(data["current_epoch"], 0)
        self.assertEqual(data["metrics"]["loss"], "0.5")
        self.assertEqual(data["metrics"]["learning_rate"], "0.001")
        self.assertEqual(data["metrics"]["accuracy"], "0.85")

    def test_json_file_format_structure(self):
        """Test that the JSON file format matches expected structure."""
        tracker = ProgressionTracker(
            total_epochs=3, steps_per_epoch=100, status_file_path=self.temp_file
        )

        tracker.update_step(epoch=1, step=50, loss=0.3)

        with open(self.temp_file, "r") as f:
            data = json.load(f)

        # Verify required fields exist
        required_fields = [
            "current_step",
            "total_steps",
            "current_epoch",
            "total_epochs",
            "message",
            "metrics",
            "timestamp",
        ]

        for field in required_fields:
            self.assertIn(field, data, f"Missing required field: {field}")

        # Verify data types
        self.assertIsInstance(data["current_step"], int)
        self.assertIsInstance(data["total_steps"], int)
        self.assertIsInstance(data["current_epoch"], int)
        self.assertIsInstance(data["total_epochs"], int)
        self.assertIsInstance(data["message"], str)
        self.assertIsInstance(data["metrics"], dict)
        self.assertIsInstance(data["timestamp"], (int, float))


if __name__ == "__main__":
    unittest.main()
