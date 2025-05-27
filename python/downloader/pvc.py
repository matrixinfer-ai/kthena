import os
from pathlib import Path
from base import ModelDownloader


class PVCDownloader(ModelDownloader):
    def __init__(self):
        super().__init__()
        self.pvc_mount_point = "/mnt/pvc"

    @staticmethod
    def _copy_from_pvc(pvc_path: str, output_dir: str):
        command = f"rsync -av --partial --progress {pvc_path}/ {output_dir}/"
        if os.system(command) != 0:
            raise RuntimeError(f"Failed to copy files from PVC path: {pvc_path}")

    def _parse_pvc_path(self) -> str:
        return str(Path(self.pvc_mount_point))

    def download(self, output_dir: str):
        pvc_path = self._parse_pvc_path()
        os.makedirs(output_dir, exist_ok=True)
        if not os.path.exists(pvc_path):
            raise FileNotFoundError(f"PVC path does not exist: {pvc_path}")
        self._copy_from_pvc(pvc_path, output_dir)
