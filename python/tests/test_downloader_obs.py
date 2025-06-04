import unittest
from unittest.mock import MagicMock, patch

from base import get_downloader
from downloader import download_model
from objectstorage import OBSDownloader


class TestDownloadModel(unittest.TestCase):
    def setUp(self):
        self.source = "obs://fake_bucket/fake_path"
        self.output_dir = "/tmp/models"
        self.model_name = "fake_name"
        self.credentials = {"access_key": "fake_ak", "secret_key": "fake_sk"}
        self.mock_downloader = MagicMock()

    def tearDown(self):
        pass

    @patch("downloader.get_downloader")
    def test_download_model_obs(self, mock_get_downloader):
        mock_get_downloader.return_value = self.mock_downloader

        download_model(self.source, self.output_dir, self.model_name, self.credentials)

        mock_get_downloader.assert_called_once_with(self.source, self.credentials)
        self.mock_downloader.download_model.assert_called_once_with(
            self.output_dir, self.model_name
        )

    @patch("downloader.get_downloader")
    def test_obs_download_model_invalid_source(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("bucketName is empty")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("bucketName is empty", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    @patch("downloader.get_downloader")
    def test_obs_download_model_invalid_access_key(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("InvalidAccessKeyId")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("InvalidAccessKeyId", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    @patch("downloader.get_downloader")
    def test_obs_download_model_invalid_secret_key(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("SignatureDoesNotMatch")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("SignatureDoesNotMatch", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    @patch("downloader.get_downloader")
    def test_obs_download_model_network_error(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ConnectionError("Gateway Time-out")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ConnectionError) as context:
            download_model(self.source, self.output_dir, self.model_name, self.credentials)

        self.assertIn("Gateway Time-out", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials)

    def test_get_obs_downloader(self):
        downloader = get_downloader(self.source, self.credentials)
        self.assertIsInstance(downloader, OBSDownloader, "Should return OBSDownloader instance")


if __name__ == "__main__":
    unittest.main()
