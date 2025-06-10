# Copyright 2025.
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

import boto3
from boto3.s3.transfer import TransferConfig
from botocore.exceptions import ClientError
import os.path

from base import ModelDownloader, parse_bucket_from_model_url
from logger import setup_logger

logger = setup_logger()


class S3Downloader(ModelDownloader):
    def __init__(self, model_uri: str, access_key: str = None, secret_key: str = None,
                 endpoint: str = None, max_workers: int = 8, use_threads: bool = True,
                 chunk_size: int = 10485760, multipart_threshold: int = 20971520):
        super().__init__()
        self.access_key = access_key
        self.secret_key = secret_key
        self.model_uri = model_uri
        self.max_concurrency = max_workers
        self.use_threads = use_threads
        self.chunk_size = chunk_size
        self.multipart_threshold = multipart_threshold
        self.client = boto3.client(
                's3',
                aws_access_key_id=access_key,
                aws_secret_access_key=secret_key,
                endpoint_url=endpoint
            )

    def download(self, output_dir: str):
        logger.info("Downloading model from S3")
        if not self.access_key or not self.secret_key:
            logger.error("Missing S3 credentials. Please provide 'access_key' and 'secret_key'.")
            return

        bucket_name, bucket_path = parse_bucket_from_model_url(self.model_uri, "s3")
        config = TransferConfig(
            use_threads=self.use_threads,  # 启用多线程
            max_concurrency=self.max_concurrency,  # 并发线程数
            multipart_threshold=self.multipart_threshold,
            multipart_chunksize=self.chunk_size
        )
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
                if os.path.exists(output_path) and os.path.getsize(output_path) == obj['Size']:
                    logger.info(f"{output_path} already exist,no need to download")
                    continue
                os.makedirs(os.path.dirname(output_path), exist_ok=True)
                self.client.download_file(bucket_name, key, output_path, Config=config)
            logger.info(f"Successfully downloaded model '{self.model_uri}' to '{output_dir}'.")
        except ClientError as e:
            error_code = e.response["Error"]["Code"]
            logger.error(f"Error downloading model '{self.model_uri}': {error_code}")
            raise
