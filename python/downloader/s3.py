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

import hashlib
import os
import re

import boto3
from boto3.s3.transfer import TransferConfig
from botocore.exceptions import ClientError

from base import ModelDownloader, parse_bucket_from_model_url
from logger import setup_logger

logger = setup_logger()


class S3Downloader(ModelDownloader):
    def __init__(self, model_uri: str, access_key: str = None, secret_key: str = None,
                 endpoint: str = None, max_workers: int = 8, chunk_size: int = 10485760):
        super().__init__()
        self.access_key = access_key
        self.secret_key = secret_key
        self.endpoint = endpoint
        self.model_uri = model_uri
        self.max_workers = max_workers
        self.chunk_size = chunk_size

        self.client = boto3.client(
            's3',
            aws_access_key_id=access_key,
            aws_secret_access_key=secret_key,
            endpoint_url=endpoint
        )

        self.transfer_config = TransferConfig(
            max_concurrency=max_workers,
            multipart_threshold=chunk_size * 5,
            multipart_chunksize=chunk_size,
            use_threads=True
        )

    @staticmethod
    def _calculate_file_md5(file_path):
        md5_hash = hashlib.md5()
        with open(file_path, 'rb') as file:
            for chunk in iter(lambda: file.read(65536), b''):
                md5_hash.update(chunk)
        return f'"{md5_hash.hexdigest()}"'

    @staticmethod
    def _is_etag_match(local_md5, etag):
        if "-" in etag:
            match = re.match(r'"([^"]+)-(\d+)"', etag)
            if match:
                return True
        return local_md5 == etag

    def _download_object(self, bucket_name, key, output_path, size, etag):
        if os.path.exists(output_path):
            if os.path.getsize(output_path) == size:
                file_md5 = self._calculate_file_md5(output_path)
                if self._is_etag_match(file_md5, etag):
                    logger.info(f"Skipping existing file: {output_path}")
                    return False

        os.makedirs(os.path.dirname(output_path), exist_ok=True)

        try:
            self.client.download_file(
                Bucket=bucket_name,
                Key=key,
                Filename=output_path,
                Config=self.transfer_config
            )
            logger.info(f"Downloaded: {key} to {output_path}")
            return True
        except ClientError as e:
            error_code = e.response.get('Error', {}).get('Code', 'Unknown')
            error_message = e.response.get('Error', {}).get('Message', str(e))
            logger.error(f"Download failed: {key}, error: {error_code} - {error_message}")
            return False

    def download(self, output_dir: str):
        logger.info(f"Downloading model from S3: {self.model_uri}")
        if not self.access_key or not self.secret_key:
            logger.error("Missing S3 credentials")
            return
        bucket_name, bucket_path = parse_bucket_from_model_url(self.model_uri, "s3")
        try:
            paginator = self.client.get_paginator('list_objects_v2')
            page_iterator = paginator.paginate(
                Bucket=bucket_name,
                Prefix=bucket_path,
                PaginationConfig={'PageSize': 1000}
            )

            for page in page_iterator:
                if 'Contents' not in page:
                    logger.warning(f"No objects found in {bucket_name}/{bucket_path}")
                    continue

                for obj in page['Contents']:
                    key = obj['Key']
                    if key.endswith("/"):
                        continue

                    size = obj['Size']
                    etag = obj['ETag']

                    rel_path = os.path.relpath(key, bucket_path) if bucket_path else key
                    output_path = os.path.join(output_dir, rel_path)

                    self._download_object(bucket_name, key, output_path, size, etag)

            logger.info(f"Download complete for model {self.model_uri}")

        except ClientError as e:
            error_code = e.response.get('Error', {}).get('Code', 'Unknown')
            error_message = e.response.get('Error', {}).get('Message', str(e))
            logger.error(f"Error listing objects: {error_code} - {error_message}")

            if error_code == 'NoSuchBucket':
                logger.error(f"Bucket {bucket_name} does not exist")
            elif error_code == 'AccessDenied':
                logger.error("Access denied to S3 bucket. Check your credentials and permissions.")

        except Exception as e:
            logger.error(f"Error downloading model: {str(e)}")
            raise
