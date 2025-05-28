import argparse
import json
import os
from pathlib import Path

from downloader import download_model
from logger import setup_logger

logger = setup_logger()


def load_credentials(credentials_str: str = None) -> dict:
    credentials = {}
    if credentials_str:
        try:
            credentials = json.loads(credentials_str)
            logger.info("Loaded credentials from JSON string.")
        except json.JSONDecodeError as e:
            logger.error(f"Error parsing credentials JSON: {e}")
            raise ValueError("Invalid credentials JSON format.") from e

    env_credentials = {
        "hf_token": os.getenv("HF_AUTH_TOKEN"),
        "hf_endpoint": os.getenv("HF_ENDPOINT"),
        "hf_revision": os.getenv("HF_REVISION"),
        "access_key": os.getenv("ACCESS_KEY"),
        "secret_key": os.getenv("SECRET_KEY"),
        "region_name": os.getenv("REGION_NAME"),
        "obs_endpoint": os.getenv("OBS_ENDPOINT"),
    }
    for key, value in env_credentials.items():
        if value:
            credentials.setdefault(key, value)
            logger.info(f"Loaded {key} from environment variables.")
    return credentials


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Universal Downloader Tool",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter
    )

    parser.add_argument(
        "-s", "--source",
        type=str,
        required=True,
        help="Source paths (e.g., HF repo IDs or S3 URIs)"
    )
    parser.add_argument(
        "-o", "--output-dir",
        type=str,
        default="~/downloads",
        help="Output directory path"
    )
    parser.add_argument(
        "-m", "--model-name",
        type=str,
        required=True,
        help="Name of the model to be downloaded"
    )
    parser.add_argument(
        "-c", "--credentials",
        type=str,
        default=None,
        help="JSON-formatted string containing authentication credentials "
             "(e.g., '{\"hf_token\": \"YOUR_HF_TOKEN\", \"access_key\": \"YOUR_ACCESS_KEY\", "
             "\"secret_key\": \"YOUR_SECRET_KEY\", \"region_name\": \"YOUR_REGION_NAME\"}')"
    )
    args = parser.parse_args()
    args.output_dir = str(Path(args.output_dir).expanduser().resolve())
    logger.info(f"Resolved output directory: {args.output_dir}")
    return args


def main():
    try:
        args = parse_arguments()
        credentials = load_credentials(args.credentials)
        download_model(
            source=args.source,
            output_dir=args.output_dir,
            model_name=args.model_name,
            credentials=credentials
        )
    except Exception as e:
        logger.error(f"An error occurred: {e}")
        exit(1)


if __name__ == "__main__":
    main()
