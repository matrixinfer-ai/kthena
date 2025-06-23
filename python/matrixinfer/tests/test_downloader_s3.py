import unittest
from unittest.mock import MagicMock, patch

from matrixinfer.downloader.base import get_downloader
from matrixinfer.downloader.downloader import download_model
from matrixinfer.downloader.s3 import S3Downloader


class TestDownloadModel(unittest.TestCase):
    def setUp(self):
        self.source = "s3://fake_bucket/fake_path"
        self.output_dir = "/tmp/models"
        self.credentials = {"access_key": "fake_ak", "secret_key": "fake_sk"}
        self.mock_downloader = MagicMock()

    def tearDown(self):
        pass

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_download_model_s3(self, mock_get_downloader):
        mock_get_downloader.return_value = self.mock_downloader

        download_model(self.source, self.output_dir, self.credentials)

        mock_get_downloader.assert_called_once_with(self.source, self.credentials, 8)
        self.mock_downloader.download_model.assert_called_once_with(
            self.output_dir
        )

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_s3_download_model_invalid_source(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("Invalid bucket name")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.credentials)

        self.assertIn("Invalid bucket name", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials, 8)

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_s3_download_model_invalid_access_key(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("InvalidAccessKeyId")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.credentials)

        self.assertIn("InvalidAccessKeyId", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials, 8)

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_s3_download_model_invalid_secret_key(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ValueError("SignatureDoesNotMatch")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ValueError) as context:
            download_model(self.source, self.output_dir, self.credentials)

        self.assertIn("SignatureDoesNotMatch", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials, 8)

    @patch("matrixinfer.downloader.downloader.get_downloader")
    def test_s3_download_model_network_error(self, mock_get_downloader):
        self.mock_downloader.download_model.side_effect = ConnectionError("Failed to establish connection to server")
        mock_get_downloader.return_value = self.mock_downloader

        with self.assertRaises(ConnectionError) as context:
            download_model(self.source, self.output_dir, self.credentials)

        self.assertIn("Failed to establish connection to server", str(context.exception))
        mock_get_downloader.assert_called_once_with(self.source, self.credentials, 8)

    @patch('matrixinfer.downloader.s3.S3Downloader._build_sync_command')
    @patch('subprocess.Popen')
    def test_s3_download_implementation(self, mock_popen, mock_build_cmd):
        process_mock = MagicMock()
        process_mock.stdout.readline.side_effect = ['output1', 'output2', '']
        process_mock.poll.return_value = 0
        process_mock.stderr.readlines.return_value = []
        mock_popen.return_value = process_mock
        mock_build_cmd.return_value = ['aws', 's3', 'sync', 's3://fake_bucket/fake_path', '/tmp/models']

        downloader = S3Downloader(
            model_uri=self.source,
            access_key="fake_ak",
            secret_key="fake_sk",
            endpoint=None,
        )

        downloader.download(self.output_dir)
        mock_build_cmd.assert_called_once_with('s3://fake_bucket/fake_path', self.output_dir)
        mock_popen.assert_called_once()

    def test_get_s3_downloader(self):
        downloader = get_downloader(self.source, self.credentials)
        self.assertIsInstance(downloader, S3Downloader, "Should return S3Downloader instance")


if __name__ == "__main__":
    unittest.main()
