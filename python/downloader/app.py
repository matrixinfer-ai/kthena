import argparse

def __main__():
    parser = argparse.ArgumentParser(
        description="Universal Downloader Tool",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter
    )
    parser.add_argument(
        "-s", "--sources",
        nargs="+",
        required=True,
        help="Source paths (HF repo IDs or S3 URIs)"
    )
    parser.add_argument(
        "-o", "--output-dir",
        default="~/downloads",
        help="Output directory path"
    )
    
    args = parser.parse_args()
    print("Hello World!")