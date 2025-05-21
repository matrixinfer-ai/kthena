from base import get_downloader


def download_model(sources: list, output_dir: str, model_name: str, credentials: dict):
    for source in sources:
        downloader = get_downloader(source, credentials)
        downloader.download_model(output_dir, model_name)