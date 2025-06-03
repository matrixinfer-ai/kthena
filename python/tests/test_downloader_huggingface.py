import unittest
from unittest.mock import MagicMock, patch

from base import get_downloader
from downloader import download_model
from huggingface import HuggingFaceDownloader


class TestDownloadModel(unittest.TestCase):
    def setUp(self):
        self.source = "fake_repo/fake_name"
        self.output_dir = "/tmp/models"
        self.model_name = "fake_name"
        self.credentials = {"hf_token": "fake-token"}
        self.mock_downloader = MagicMock()

    def tearDown(self):
        pass

    @patch("downloader.get_downloader")
    def test_download_model_huggingface(self, mock_get_downloader):
        mock_get_downloader.return_value = self.mock_downloader

        download_model(self.source, self.output_dir, self.model_name, self.credentials)

        mock_get_downloader.assert_called_once_with(self.source, self.credentials)
        self.mock_downloader.download_model.assert_called_once_with(
            self.output_dir, self.model_name
        )

    @patch("downloader.get_downloader")
    def test_download_model_invalid_credentials(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("Invalid authentication token")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("Invalid authentication token", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    @patch("downloader.get_downloader")
    def test_download_model_network_error(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ConnectionError("Failed to establish connection to server")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ConnectionError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("Failed to establish connection to server", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    @patch("downloader.get_downloader")
    def test_download_model_permission_error(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = PermissionError(
            "Insufficient permissions to write to output directory")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(PermissionError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("Insufficient permissions", str(context.exception))

    def test_get_downloader_huggingface(self):
        downloader = get_downloader(self.source, self.credentials)
        self.assertIsInstance(downloader, HuggingFaceDownloader, "Should return HuggingFaceDownloader instance")


if __name__ == "__main__":
    unittest.main()
