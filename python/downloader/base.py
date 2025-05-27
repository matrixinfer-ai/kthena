import os
import threading
from abc import ABC, abstractmethod

from lock import LockManager
from logger import setup_logger

logger = setup_logger()


class ModelDownloader(ABC):
    def __init__(self):
        self.lock_manager = None
        self.renew_thread = None
        self.stop_renew = threading.Event()

    @abstractmethod
    def download(self, output_dir: str):
        """
        Abstract method to be implemented by subclasses.
        This method should define the actual download logic.
        """
        pass

    def download_model(self, output_dir: str, model_name: str):
        output_dir = os.path.join(output_dir, model_name)
        os.makedirs(output_dir, exist_ok=True)
        lock_path = os.path.join(output_dir, f".{model_name}.lock")
        self.lock_manager = LockManager(lock_path, timeout=600)
        while True:
            if self.lock_manager.try_acquire():
                try:
                    self.stop_renew.clear()
                    self.renew_thread = threading.Thread(
                        target=self.lock_manager.renew,
                        args=(300,),
                        daemon=True
                    )
                    self.renew_thread.start()
                    logger.info(f"Acquired lock successfully. Starting download for model '{model_name}'")
                    self.download(output_dir)
                    break
                finally:
                    self.lock_manager.stop_renew()
                    if self.renew_thread and self.renew_thread.is_alive():
                        self.renew_thread.join(timeout=3)
                    self.lock_manager.release()
            else:
                logger.info("Failed to acquire lock. Waiting for the lock to be released.")
                self.stop_renew.wait(timeout=60)


def get_downloader(url: str, credentials: dict) -> ModelDownloader:
    try:
        if url.startswith("s3://"):
            from s3 import S3Downloader
            return S3Downloader(
                model_uri=url,
                access_key=credentials.get("access_key"),
                secret_key=credentials.get("secret_key"),
                s3_endpoint=credentials.get("s3_endpoint"),
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
        logger.error(f"Failed to initialize downloader: {str(e)}")
        raise RuntimeError(f"Failed to initialize downloader: {str(e)}")
