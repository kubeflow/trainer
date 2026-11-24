# Copyright The Kubeflow Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Tests for OpenDAL utilities."""

import fnmatch
import os
import tempfile
from unittest.mock import MagicMock, Mock, patch

import pytest
from opendal.exceptions import NotFound

from pkg.initializers.utils.opendal import S3Storage


class TestS3Storage:
    """Test suite for S3Storage class."""

    @pytest.mark.parametrize(
        "config,expected_region",
        [
            (
                {
                    "bucket": "test-bucket",
                    "endpoint": "https://s3.us-west-2.amazonaws.com",
                    "access_key_id": "test_key_id",
                    "secret_access_key": "test_secret_key",
                    "region": "us-west-2",
                    "role_arn": "arn:aws:iam::123456789012:role/TestRole",
                },
                "us-west-2",
            ),
            (
                {
                    "bucket": "prod-bucket",
                    "endpoint": "https://s3.eu-west-1.amazonaws.com",
                    "access_key_id": "prod_key",
                    "secret_access_key": "prod_secret",
                    "role_arn": "arn:aws:iam::123456789012:role/TestRole",
                },
                "auto",
            ),
        ],
    )
    def test_init(self, config, expected_region):
        """Test S3Storage initialization with various configurations."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator

            storage = S3Storage(**config)

            # Verify bucket is stored correctly
            assert storage.bucket == config["bucket"]

            # Verify Operator was called with correct parameters
            call_kwargs = mock_opendal.Operator.call_args[1]

            # Check required parameters
            assert call_kwargs["bucket"] == config["bucket"]
            assert call_kwargs["region"] == expected_region

            # Check optional parameters if provided
            for key in ["endpoint", "access_key_id", "secret_access_key"]:
                if key in config:
                    assert call_kwargs[key] == config[key]

    def _create_mock_entry(self, path: str, is_dir: bool = False) -> Mock:
        """Helper to create a mock S3 entry."""
        mock_entry = Mock()
        mock_entry.path = path
        mock_entry.metadata.is_dir = is_dir
        return mock_entry

    @pytest.mark.parametrize(
        "prefix,files,ignore_patterns",
        [
            (
                "dataset/",
                {
                    "dataset/train/data.csv": b"train data",
                    "dataset/test/data.csv": b"test data",
                    "dataset/README.md": b"readme content",
                },
                [],
            ),
            (
                "models/",
                {
                    "models/model.pth": b"model weights",
                    "models/config.json": b"config data",
                },
                [],
            ),
            (
                "data/",
                {
                    "data/file.txt": b"content",
                    "data/temp.log": b"log content",
                    "data/cache.tmp": b"cache content",
                },
                ["*.log", "*.tmp"],
            ),
        ],
    )
    def test_download(self, prefix, files, ignore_patterns):
        """Test downloading files from S3."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator

            # Setup mock entries
            mock_entries = [self._create_mock_entry(path) for path in files.keys()]
            mock_operator.list.return_value = mock_entries

            # Setup read side effect
            mock_operator.read.side_effect = list(files.values())

            storage = S3Storage(bucket="test-bucket")

            with tempfile.TemporaryDirectory() as tmpdir:
                storage.download(
                    prefix=prefix,
                    destination_path=tmpdir,
                    ignore_patterns=ignore_patterns,
                )

                # Verify downloaded files
                for path, expected_content in files.items():
                    relative_path = path[len(prefix) :].lstrip("/")
                    full_path = os.path.join(tmpdir, relative_path)

                    # Check if file should be ignored
                    should_ignore = any(
                        fnmatch.fnmatch(path, pattern) for pattern in ignore_patterns
                    )

                    if should_ignore:
                        assert not os.path.exists(
                            full_path
                        ), f"File {relative_path} should have been ignored"
                    else:
                        assert os.path.exists(
                            full_path
                        ), f"Expected file {relative_path} not found"

                        with open(full_path, "rb") as f:
                            actual_content = f.read()
                            assert (
                                actual_content == expected_content
                            ), f"Content mismatch for {relative_path}"

    def _mock_bucket(self, mock_operator: MagicMock, objects: dict[str, bytes]):
        """Make the mock operator behave like OpenDAL over a bucket of objects.

        ``list`` matches the given path as a plain string prefix, like S3
        ListObjectsV2, and ``stat`` succeeds only for an exact object key.
        """

        def list_objects(path, recursive=False):
            prefix = "" if path == "/" else path
            return [
                self._create_mock_entry(key)
                for key in objects
                if key.startswith(prefix)
            ]

        def stat(path):
            if path in objects:
                return Mock(is_file=True, is_dir=False)
            raise NotFound(f"{path} not found")

        mock_operator.list.side_effect = list_objects
        mock_operator.stat.side_effect = stat
        mock_operator.read.side_effect = lambda key: objects[key]

    def _downloaded_files(self, directory: str) -> list[str]:
        return sorted(
            os.path.relpath(os.path.join(root, name), directory).replace(os.sep, "/")
            for root, _, names in os.walk(directory)
            for name in names
        )

    @pytest.mark.parametrize("prefix", ["models/llama", "models/llama/"])
    def test_download_only_objects_under_prefix(self, prefix):
        """Objects that merely share the prefix as a string are not downloaded."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator
            self._mock_bucket(
                mock_operator,
                {
                    "models/llama/config.json": b"config",
                    "models/llama/weights/model.safetensors": b"weights",
                    "models/llama-2/config.json": b"sibling",
                    "models/llama2-7b/config.json": b"sibling",
                },
            )

            storage = S3Storage(bucket="test-bucket", region="us-east-1")

            with tempfile.TemporaryDirectory() as tmpdir:
                storage.download(prefix=prefix, destination_path=tmpdir)
                downloaded = self._downloaded_files(tmpdir)

            assert downloaded == ["config.json", "weights/model.safetensors"]
            # The directory is listed with a trailing slash, so the siblings are
            # never returned by the storage, let alone read.
            mock_operator.list.assert_called_once_with("models/llama/", recursive=True)
            assert sorted(
                call.args[0] for call in mock_operator.read.call_args_list
            ) == [
                "models/llama/config.json",
                "models/llama/weights/model.safetensors",
            ]

    def test_download_single_object(self):
        """A prefix that names one object downloads that object under its file name."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator
            self._mock_bucket(
                mock_operator,
                {
                    "datasets/train.parquet": b"rows",
                    "datasets/train.parquet.bak": b"stale rows",
                },
            )

            storage = S3Storage(bucket="test-bucket", region="us-east-1")

            with tempfile.TemporaryDirectory() as tmpdir:
                storage.download(
                    prefix="datasets/train.parquet", destination_path=tmpdir
                )

                assert os.listdir(tmpdir) == ["train.parquet"]
                with open(os.path.join(tmpdir, "train.parquet"), "rb") as f:
                    assert f.read() == b"rows"

            # The exact key is read directly, nothing is listed.
            mock_operator.list.assert_not_called()
            mock_operator.read.assert_called_once_with("datasets/train.parquet")

    def test_download_exact_object_takes_precedence(self):
        """When both an object and a directory have the name, the object wins."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator
            self._mock_bucket(
                mock_operator,
                {
                    "models/llama": b"archive",
                    "models/llama/config.json": b"config",
                },
            )

            storage = S3Storage(bucket="test-bucket", region="us-east-1")

            with tempfile.TemporaryDirectory() as tmpdir:
                storage.download(prefix="models/llama", destination_path=tmpdir)
                assert self._downloaded_files(tmpdir) == ["llama"]

            with tempfile.TemporaryDirectory() as tmpdir:
                storage.download(prefix="models/llama/", destination_path=tmpdir)
                assert self._downloaded_files(tmpdir) == ["config.json"]

    def test_download_bucket_root(self):
        """An empty prefix (s3://bucket) downloads the whole bucket."""
        objects = {
            "README.md": b"readme",
            "datasets/train.parquet": b"rows",
            "models/llama/config.json": b"config",
        }
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator
            self._mock_bucket(mock_operator, objects)

            storage = S3Storage(bucket="test-bucket", region="us-east-1")

            with tempfile.TemporaryDirectory() as tmpdir:
                storage.download(prefix="", destination_path=tmpdir)
                assert self._downloaded_files(tmpdir) == sorted(objects)

            mock_operator.stat.assert_not_called()
            mock_operator.list.assert_called_once_with("/", recursive=True)

    @pytest.mark.parametrize(
        "prefix,objects,expected_message",
        [
            (
                # A partial object name no longer matches anything.
                "datasets/train",
                {"datasets/train-00001.parquet": b"rows"},
                "No object at s3://test-bucket/datasets/train and no objects under "
                "s3://test-bucket/datasets/train/",
            ),
            (
                "datasets/train/",
                {"datasets/train-00001.parquet": b"rows"},
                "No objects under s3://test-bucket/datasets/train/",
            ),
            (
                "",
                {},
                "No objects under s3://test-bucket/",
            ),
        ],
    )
    def test_download_nothing_found(self, prefix, objects, expected_message):
        """A prefix that matches no object fails instead of succeeding silently."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator
            self._mock_bucket(mock_operator, objects)

            storage = S3Storage(bucket="test-bucket", region="us-east-1")

            with tempfile.TemporaryDirectory() as tmpdir:
                with pytest.raises(FileNotFoundError, match=expected_message):
                    storage.download(prefix=prefix, destination_path=tmpdir)
                assert os.listdir(tmpdir) == []

            mock_operator.read.assert_not_called()

    def test_download_all_objects_ignored_warns(self, caplog):
        """Ignore patterns that skip every object log a warning but do not fail."""
        with patch("pkg.initializers.utils.opendal.opendal") as mock_opendal:
            mock_operator = MagicMock()
            mock_opendal.Operator.return_value = mock_operator
            mock_operator.layer.return_value = mock_operator
            self._mock_bucket(mock_operator, {"data/cache.tmp": b"cache"})

            storage = S3Storage(bucket="test-bucket", region="us-east-1")

            with tempfile.TemporaryDirectory() as tmpdir:
                with caplog.at_level("WARNING"):
                    storage.download(
                        prefix="data/",
                        destination_path=tmpdir,
                        ignore_patterns=["*.tmp"],
                    )
                assert os.listdir(tmpdir) == []

            assert "skipped by the ignore patterns" in caplog.text
            mock_operator.read.assert_not_called()
