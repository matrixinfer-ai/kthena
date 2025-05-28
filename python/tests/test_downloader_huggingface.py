import unittest
from unittest.mock import MagicMock, patch

from base import get_downloader
from downloader import download_model
from huggingface import HuggingFaceDownloader


class TestDownloadModel(unittest.TestCase):

    @patch("downloader.get_downloader")
    def test_download_model_huggingface(self, mock_get_downloader):
        mock_downloader = MagicMock()
        mock_get_downloader.return_value = mock_downloader

        source = "fake_repo/fake_name"
        output_dir = "/tmp/models"
        model_name = "fake_name"
        credentials = {"hf_token": "fake-token"}

        download_model(source, output_dir, model_name, credentials)
        mock_get_downloader.assert_called_once_with("fake_repo/fake_name", credentials)
        mock_downloader.download_model.assert_called_once_with(output_dir, model_name)

    @patch("downloader.get_downloader")
    def test_download_model_invalid_credentials(self, mock_get_downloader):
        mock_downloader = MagicMock()
        mock_downloader.download_model.side_effect = ValueError("Invalid credentials")
        mock_get_downloader.return_value = mock_downloader

        source = "fake_repo/fake_name"
        output_dir = "/tmp/models"
        model_name = "fake_name"
        credentials = {"hf_token": "invalid-token"}

        with self.assertRaises(ValueError) as context:
            download_model(source, output_dir, model_name, credentials)
        self.assertIn("Invalid credentials", str(context.exception))

    @patch("downloader.get_downloader")
    def test_download_model_network_error(self, mock_get_downloader):
        mock_downloader = MagicMock()
        mock_downloader.download_model.side_effect = ConnectionError("Network error")
        mock_get_downloader.return_value = mock_downloader

        source = "fake_repo/fake_name"
        output_dir = "/tmp/models"
        model_name = "fake_name"
        credentials = {"hf_token": "fake-token"}

        with self.assertRaises(ConnectionError) as context:
            download_model(source, output_dir, model_name, credentials)
        self.assertIn("Network error", str(context.exception))

    def test_get_downloader_huggingface(self):
        credentials = {"hf_token": "fake-token"}
        source = "fake_repo/fake_name"
        downloader = get_downloader(source, credentials)
        assert isinstance(downloader, HuggingFaceDownloader)


if __name__ == "__main__":
    unittest.main()
