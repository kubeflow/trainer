import os
import shutil
import tempfile

import pytest


@pytest.fixture
def setup_temp_path(monkeypatch):
    """Creates temporary directory and patches path constant.

    This fixture:
    1. Creates a temporary directory
    2. Allows configuration of path constant
    3. Handles automatic cleanup after tests
    4. Restores original environment state

    Args:
        monkeypatch: pytest fixture for modifying objects

    Returns:
        function: A configurator that accepts path_var (str) and returns temp_dir path

    Usage:
        def test_something(setup_temp_path):
            temp_dir = setup_temp_path("MODEL_PATH")
            # temp_dir is created and MODEL_PATH is patched
            # cleanup happens automatically after test
    """
    # Setup
    original_env = dict(os.environ)
    current_dir = os.path.dirname(os.path.abspath(__file__))
    temp_dir = tempfile.mkdtemp(dir=current_dir)

    def configure_path(path_var: str):
        """Configure path variable in kubeflow.trainer"""
        import kubeflow.trainer as trainer

        monkeypatch.setattr(trainer, path_var, temp_dir)
        return temp_dir

    yield configure_path

    # Cleanup temp directory after test
    shutil.rmtree(temp_dir, ignore_errors=True)
    os.environ.clear()
    os.environ.update(original_env)


def verify_downloaded_files(dir_path, expected_files):
    """Verify downloaded files"""
    if expected_files:
        actual_files = set(os.listdir(dir_path))
        missing_files = set(expected_files) - actual_files
        assert not missing_files, f"Missing expected files: {missing_files}"
