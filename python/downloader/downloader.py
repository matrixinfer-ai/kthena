from base import get_downloader


def download_model(source: str, output_dir: str, model_name: str, credentials: dict):
    downloader = get_downloader(source, credentials)
    downloader.download_model(output_dir, model_name)