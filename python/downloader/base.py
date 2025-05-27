import os
import threading
from abc import ABC, abstractmethod
from typing import Optional

from lock import LockManager, LockError
from logger import setup_logger

logger = setup_logger()


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
            except LockError as e:
                logger.error(f"Lock error: {e}")
                if self.lock_manager:
                    self.lock_manager.release()
                raise
            except Exception as e:
                logger.error(f"Unexpected error in download_model: {e}")
                if self.lock_manager:
                    self.lock_manager.release()
                raise


def get_downloader(url: str, credentials: dict) -> ModelDownloader:
    try:
        if url.startswith("s3://"):
            from s3 import S3Downloader
            return S3Downloader(
                model_uri=url,
                access_key=credentials.get("access_key"),
                secret_key=credentials.get("secret_key"),
                region_name=credentials.get("region_name"),
            )
        elif url.startswith("pvc://"):
            from pvc import PVCDownloader
            return PVCDownloader()
        else:
            from huggingface import HuggingFaceDownloader
            return HuggingFaceDownloader(
                model_uri=url,
                hf_token=credentials.get("hf_token"),
                hf_endpoint=credentials.get("hf_endpoint"),
                hf_revision=credentials.get("hf_revision"),
            )
    except ImportError as e:
        logger.error(f"Failed to initialize downloader: {e}")
        raise RuntimeError(f"Failed to initialize downloader: {e}")
