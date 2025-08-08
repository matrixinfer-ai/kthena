# Copyright MatrixInfer-AI Authors.
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

import unittest
from unittest.mock import MagicMock, patch

from matrixinfer.downloader.base import get_downloader
from matrixinfer.downloader.downloader import download_model
from matrixinfer.downloader.huggingface import HuggingFaceDownloader


class TestDownloadModel(unittest.TestCase):
    def setUp(self):
        self.source = "fake_repo/fake_name"
        self.output_dir = "/tmp/models"
        self.credentials = {"hf_token": "fake-token"}
        self.mock_downloader = MagicMock()

    def tearDown(self):
        pass

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_download_model_huggingface_default_workers(self, mock_get_downloader):
        mock_get_downloader.return_value = self.mock_downloader

        download_model(self.source, self.output_dir, self.credentials)

        mock_get_downloader.assert_called_once_with(self.source, self.credentials, 8)
        self.mock_downloader.download_model.assert_called_once_with(self.output_dir)

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_download_model_huggingface_custom_workers(self, mock_get_downloader):
        mock_get_downloader.return_value = self.mock_downloader

        custom_max_workers = 16
        download_model(
            self.source,
            self.output_dir,
            self.credentials,
            max_workers=custom_max_workers,
        )

        mock_get_downloader.assert_called_once_with(
            self.source, self.credentials, custom_max_workers
        )
        self.mock_downloader.download_model.assert_called_once_with(self.output_dir)

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_download_model_invalid_credentials(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError(
            "Invalid authentication token"
        )
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.credentials)

        self.assertIn("Invalid authentication token", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials, 8)

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_download_model_network_error(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ConnectionError(
            "Failed to establish connection to server"
        )
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ConnectionError) as context:
            download_model(self.source, self.output_dir, self.credentials)

        self.assertIn(
            "Failed to establish connection to server", str(context.exception)
        )
        mock_get_downloader.assert_called_once_with(self.source, self.credentials, 8)

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_download_model_permission_error(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = PermissionError(
            "Insufficient permissions to write to output directory"
        )
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(PermissionError) as context:
            download_model(self.source, self.output_dir, self.credentials)

        self.assertIn("Insufficient permissions", str(context.exception))

    def test_get_downloader_huggingface_default_workers(self):
        downloader = get_downloader(self.source, self.credentials)
        self.assertIsInstance(
            downloader,
            HuggingFaceDownloader,
            "Should return HuggingFaceDownloader instance",
        )
        if isinstance(downloader, HuggingFaceDownloader):
            self.assertEqual(
                downloader.max_workers, 8, "max_workers should default to 8"
            )

    def test_get_downloader_huggingface_custom_workers(self):
        custom_max_workers = 16
        downloader = get_downloader(
            self.source, self.credentials, max_workers=custom_max_workers
        )
        self.assertIsInstance(
            downloader,
            HuggingFaceDownloader,
            "Should return HuggingFaceDownloader instance",
        )
        if isinstance(downloader, HuggingFaceDownloader):
            self.assertEqual(
                downloader.max_workers,
                custom_max_workers,
                f"max_workers should be {custom_max_workers}",
            )


if __name__ == "__main__":
    unittest.main()
