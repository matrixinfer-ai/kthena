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

import os
from obs import ObsClient

from base import ModelDownloader, parse_bucket_from_model_url
from logger import setup_logger

logger = setup_logger()


class OBSDownloader(ModelDownloader):
    def __init__(self, model_uri: str, access_key: str = None, secret_key: str = None, obs_endpoint: str = None):
        super().__init__()
        self.model_uri = model_uri
        self.access_key = access_key
        self.secret_key = secret_key
        self.client = ObsClient(
            access_key_id=access_key,
            secret_access_key=secret_key,
            server=obs_endpoint
        )

    def download(self, output_dir: str):
        logger.info("Downloading model from OBS")
        if not self.access_key or not self.secret_key:
            logger.error("Missing OBS credentials. Please provide 'access_key' and 'secret_key'.")
            return
        bucket_name, bucket_path = parse_bucket_from_model_url(self.model_uri, "obs")
        try:
            resp = self.client.listObjects(bucket_name, prefix=bucket_path)
            if resp.status > 300:
                logger.error(f"list status:{resp.status},code:{resp.errorCode},message: {resp.errorMessage}")
                return
            for content in resp.body.contents:
                if not content.key.endswith('/'):
                    output_path = os.path.join(output_dir, os.path.relpath(content.key, bucket_path))
                    get_resp = self.client.getObject(bucket_name, content.key, downloadPath=output_path)
                    if get_resp.status > 300:
                        logger.error(f"get status:{resp.status},code:{resp.errorCode},message: {resp.errorMessage}")
            logger.info(f"Successfully downloaded model '{self.model_uri}' to '{output_dir}'.")
        except Exception as e:
            logger.error(f"Error downloading model '{self.model_uri}': {e}")
            raise


