import os
import subprocess
from pathlib import Path
from base import ModelDownloader
from logger import setup_logger

logger = setup_logger()


class PVCDownloader(ModelDownloader):
    def __init__(self):
        super().__init__()
        self.pvc_mount_point = "/mnt/pvc"

    @staticmethod
    def _copy_from_pvc(pvc_path: str, output_dir: str):
        try:
            pvc_path_obj = Path(pvc_path).resolve()
            output_dir_obj = Path(output_dir).resolve()
            if not pvc_path_obj.exists() or not pvc_path_obj.is_dir():
                raise FileNotFoundError(f"Invalid PVC path: {pvc_path}")
            output_dir_obj.mkdir(parents=True, exist_ok=True)
            subprocess.run(
                [
                    "rsync",
                    "-av",
                    "--partial",
                    "--progress",
                    f"{str(pvc_path_obj)}/",
                    f"{str(output_dir_obj)}/"
                ],
                check=False,
                capture_output=True,
                text=True
            )
        except Exception as e:
            logger.error(f"Error copying files from PVC: {str(e)}")
            raise

    def _parse_pvc_path(self) -> str:
        return os.path.normpath(self.pvc_mount_point)

    def download(self, output_dir: str):
        pvc_path = self._parse_pvc_path()
        os.makedirs(output_dir, exist_ok=True)

        if not os.path.exists(pvc_path):
            logger.error(f"PVC path does not exist: {pvc_path}")
            raise FileNotFoundError(f"PVC path does not exist: {pvc_path}")

        logger.info(f"Starting to copy files from PVC at {pvc_path}")
        self._copy_from_pvc(pvc_path, output_dir)
        logger.info(f"Successfully copied files to {output_dir}")
