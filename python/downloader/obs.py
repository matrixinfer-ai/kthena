import os
from obs import ObsClient, ObsException

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
            for content in resp.body.contents:
                if not content.key.endswith('/'):
                    output_path = os.path.join(output_dir, os.path.relpath(content.key, bucket_path))
                    self.client.getObject(bucket_name, content.key, downloadPath=output_path)
        except ObsException as e:
            logger.error(f"Downloading model error : {e.error_code}-{e.error_message}")
            raise
        except Exception as e:
            logger.error(f"Error downloading model '{self.model_uri}': {e}")
            raise


