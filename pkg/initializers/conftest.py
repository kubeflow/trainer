import os

import pytest


@pytest.fixture
def mock_env_vars():
    """Fixture to set and clean up environment variables"""
    original_env = dict(os.environ)

    def _set_env_vars(**kwargs):
        for key, value in kwargs.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = str(value)
        return os.environ

    yield _set_env_vars

    # Cleanup
    os.environ.clear()
    os.environ.update(original_env)
