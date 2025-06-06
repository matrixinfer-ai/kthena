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

from huggingface_hub import snapshot_download

from logger import setup_logger

logger = setup_logger()

from base import ModelDownloader


class HuggingFaceDownloader(ModelDownloader):
    def __init__(self, model_uri: str, hf_revision: str = None, hf_token: str = None, hf_endpoint: str = None,
                 force_download: bool = False, max_workers: int = 8):
        super().__init__()
        self.model_uri = model_uri
        self.hf_revision = hf_revision
        self.hf_token = hf_token
        self.hf_endpoint = hf_endpoint
        self.force_download = force_download
        self.max_workers = max_workers

    def download(self, output_dir: str):
        logger.info(f"Downloading model from Hugging Face: {self.model_uri}")
        try:
            snapshot_download(
                repo_id=self.model_uri,
                revision=self.hf_revision,
                token=self.hf_token,
                endpoint=self.hf_endpoint,
                local_dir=output_dir,
                force_download=self.force_download,
                max_workers=self.max_workers
            )
        except Exception as e:
            logger.error(f"Error downloading model '{self.model_uri}': {e}")
            raise
