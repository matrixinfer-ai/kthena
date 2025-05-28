import boto3
from botocore.exceptions import ClientError
import os.path
from typing import Tuple
from urllib.parse import urlparse

from base import ModelDownloader
from logger import setup_logger

logger = setup_logger()


def _parse_bucket_from_model_url(url: str) -> Tuple[str, str]:
    result = urlparse(url, scheme="s3")
    bucket_name = result.netloc
    bucket_path = result.path.lstrip("/")
    return bucket_name, bucket_path


class S3Downloader(ModelDownloader):
    def __init__(self, model_uri: str, access_key: str = None, secret_key: str = None, region_name: str = None):
        super().__init__()
        self.access_key = access_key
        self.secret_key = secret_key
        self.model_uri = model_uri
        self.client = boto3.client(
                's3',
                aws_access_key_id=access_key,
                aws_secret_access_key=secret_key,
                region_name=region_name
            )

    def download(self, output_dir: str):
        logger.info("Downloading model from S3")
        if not self.access_key or not self.secret_key:
            logger.error("Missing S3 credentials. Please provide 'access_key' and 'secret_key'.")
            return

        bucket_name, bucket_path = _parse_bucket_from_model_url(self.model_uri)
        try:
            list_response = self.client.list_objects_v2(Bucket=bucket_name, Prefix=bucket_path)
            obj_list = list_response.get('Contents', [])
            if not obj_list:
                logger.error("no found object in bucket")
            for obj in obj_list:
                key = obj['Key']
                if key.endswith("/"):
                    continue
                output_path = os.path.join(output_dir, os.path.relpath(key, bucket_path))
                os.makedirs(os.path.dirname(output_path), exist_ok=True)
                self.client.download_file(bucket_name, key, output_path)
            logger.info(f"Successfully downloaded model '{self.model_uri}' to '{output_dir}'.")
        except ClientError as e:
            error_code = e.response["Error"]["Code"]
            logger.error(f"Error downloading model '{self.model_uri}': {error_code}")
