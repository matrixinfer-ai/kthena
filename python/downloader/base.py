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
import threading
from abc import ABC, abstractmethod
from typing import Optional
from typing import Tuple
from urllib.parse import urlparse

from lock import LockManager, LockError
from logger import setup_logger

logger = setup_logger()


def parse_bucket_from_model_url(url: str, scheme: str) -> Tuple[str, str]:
    result = urlparse(url, scheme=scheme)
    bucket_name = result.netloc
    bucket_path = result.path.lstrip("/")
    return bucket_name, bucket_path


class ModelDownloader(ABC):
    def __init__(self):
        self.lock_manager: Optional[LockManager] = None
        self.stop_event = threading.Event()

    @abstractmethod
    def download(self, output_dir: str):
        pass

    def download_model(self, output_dir: str, model_name: str):
        output_dir = os.path.join(output_dir, model_name)
        os.makedirs(output_dir, exist_ok=True)
        lock_path = os.path.join(output_dir, f".{model_name}.lock")
        self.lock_manager = LockManager(lock_path, timeout=600)
        while True:
            try:
                if self.lock_manager.try_acquire():
                    try:
                        logger.info(f"Acquired lock successfully. Starting download for model '{model_name}'")
                        self.download(output_dir)
                        break
                    except Exception as e:
                        logger.error(f"Error during model download: {e}")
                        raise
                    finally:
                        self.lock_manager.release()
                else:
                    logger.info("Failed to acquire lock. Waiting for the lock to be released.")
                    self.stop_event.wait(timeout=60)
            except Exception as e:
                logger.error(f"Unexpected error in download_model: {e}")
                if self.lock_manager:
                    self.lock_manager.release()
                raise


def get_downloader(url: str, config: dict) -> ModelDownloader:
    try:
        if url.startswith("s3://"):
            from s3 import S3Downloader
            return S3Downloader(
                model_uri=url,
                access_key=config.get("access_key"),
                secret_key=config.get("secret_key"),
                region_name=config.get("region_name"),
            )
        elif url.startswith("pvc://"):
            from pvc import PVCDownloader
            return PVCDownloader()
        elif url.startswith("obs://"):
            from objectstorage import OBSDownloader
            return OBSDownloader(
                model_uri=url,
                access_key=config.get("access_key"),
                secret_key=config.get("secret_key"),
                obs_endpoint=config.get("obs_endpoint"),
            )
        else:
            from huggingface import HuggingFaceDownloader
            return HuggingFaceDownloader(
                model_uri=url,
                hf_token=config.get("hf_token"),
                hf_endpoint=config.get("hf_endpoint"),
                hf_revision=config.get("hf_revision"),
            )
    except ImportError as e:
        logger.error(f"Failed to initialize downloader: {e}")
        raise RuntimeError(f"Failed to initialize downloader: {e}")
