from base import ModelDownloader
from logger import setup_logger

logger = setup_logger()


class S3Downloader(ModelDownloader):
    def __init__(self, access_key: str = None, secret_key: str = None):
        super().__init__()
        self.access_key = access_key
        self.secret_key = secret_key

    def download(self, output_dir: str):
        logger.info("Downloading model from S3")
        if not self.access_key or not self.secret_key:
            logger.error("Missing S3 credentials. Please provide 'access_key' and 'secret_key'.")
            return
        # TODO: 实现下载逻辑
