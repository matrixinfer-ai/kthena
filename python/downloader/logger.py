import logging


def setup_logger():
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s - %(levelname)s - [%(filename)s:%(lineno)d] - %(message)s",
        handlers=[
            logging.StreamHandler(),
            logging.FileHandler("/tmp/matrixinfer.log")
        ]
    )
    return logging.getLogger(__name__)
