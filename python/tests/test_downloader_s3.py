import unittest
from unittest.mock import MagicMock, patch

from base import get_downloader
from downloader import download_model
from s3 import S3Downloader


class TestDownloadModel(unittest.TestCase):
    def setUp(self):
        self.source = "s3://fake_bucket/fake_path"
        self.output_dir = "/tmp/models"
        self.model_name = "fake_name"
        self.credentials = {"access_key": "fake_ak", "secret_key": "fake_sk"}
        self.mock_downloader = MagicMock()

    def tearDown(self):
        pass

    @patch("downloader.get_downloader")
    def test_download_model_s3(self, mock_get_downloader):
        mock_get_downloader.return_value = self.mock_downloader

        download_model(self.source, self.output_dir, self.model_name, self.credentials)

        mock_get_downloader.assert_called_once_with(self.source, self.credentials)
        self.mock_downloader.download_model.assert_called_once_with(
            self.output_dir, self.model_name
        )

    @patch("downloader.get_downloader")
    def test_s3_download_model_invalid_source(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("Invalid bucket name")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("Invalid bucket name", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    @patch("downloader.get_downloader")
    def test_s3_download_model_invalid_access_key(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("InvalidAccessKeyId")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("InvalidAccessKeyId", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    @patch("downloader.get_downloader")
    def test_s3_download_model_invalid_secret_key(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("SignatureDoesNotMatch")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("SignatureDoesNotMatch", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    @patch("downloader.get_downloader")
    def test_s3_download_model_network_error(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ConnectionError("Failed to establish connection to server")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ConnectionError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("Failed to establish connection to server", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    def test_get_s3_downloader(self):
        downloader = get_downloader(self.source, self.credentials)
        self.assertIsInstance(downloader, S3Downloader, "Should return S3Downloader instance")


if __name__ == "__main__":
    unittest.main()
