from huggingface_hub import snapshot_download

from logger import setup_logger

logger = setup_logger()

from base import ModelDownloader


class HuggingFaceDownloader(ModelDownloader):
    def __init__(self, model_uri: str, hf_revision: str = None, hf_token: str = None, hf_endpoint: str = None,
                 force_download: bool = False):
        super().__init__()
        self.model_uri = model_uri
        self.hf_revision = hf_revision
        self.hf_token = hf_token
        self.hf_endpoint = hf_endpoint
        self.force_download = force_download

    def download(self, output_dir: str):
        logger.info(f"Downloading model from Hugging Face: {self.model_uri}")
        try:
            snapshot_download(
                repo_id=self.model_uri,
                revision=self.hf_revision,
                token=self.hf_token,
                endpoint=self.hf_endpoint,
                local_dir=output_dir,
                force_download=self.force_download
            )
        except Exception as e:
            logger.error(f"Error downloading model '{self.model_uri}': {e}")
            raise
