import boto3
import os.path
from typing import Tuple
from urllib.parse import urlparse

from base import ModelDownloader
from logger import setup_logger

logger = setup_logger()


def _parse_bucket_from_model_url(url:str) -> Tuple[str, str]:
    result = urlparse(url, scheme="s3")
    bucket_name = result.netloc
    bucket_path = result.path.lstrip("/")
    return bucket_name, bucket_path


class S3Downloader(ModelDownloader):
    def __init__(self, model_uri: str, access_key: str = None, secret_key: str = None, s3_endpoint: str = None):
        super().__init__()
        self.access_key = access_key
        self.secret_key = secret_key
        self.model_uri = model_uri
        self.endpoint = s3_endpoint

    def download(self, output_dir: str):
        logger.info("Downloading model from S3")
        if not self.access_key or not self.secret_key:
            logger.error("Missing S3 credentials. Please provide 'access_key' and 'secret_key'.")
            return
        # download file
        bucket_name, bucket_path = _parse_bucket_from_model_url(self.model_uri)
        try:
            client = boto3.client(
                's3',
                aws_access_key_id=self.access_key,
                aws_secret_access_key=self.secret_key,
                endpoint_url=self.endpoint
            )

            paginator = client.get_paginator('list_objects_v2')
            for page in paginator.paginate(Bucket=bucket_name, Prefix=bucket_path):
                if 'Contents' not in page:
                    continue
                for obj in page['Contents']:
                    key = obj['Key']
                    if not os.path.basename(key):
                        continue
                    if output_dir:
                        output_path = os.path.join(output_dir, os.path.relpath(key, bucket_path))
                    else:
                        output_path = os.path.relpath(key, bucket_path)

                    os.makedirs(os.path.dirname(output_path), exist_ok=True)
                    client.download_file(bucket_name, key, output_path)
            logger.info(f"Successfully downloaded model '{self.model_uri}' to '{output_dir}'.")
        except Exception as e:
            logger.error(f"Error downloading model '{self.model_uri}': {e}")