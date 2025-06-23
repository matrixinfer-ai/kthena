# Copyright 2025.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import logging
import sys
import os
from logging.handlers import RotatingFileHandler


def setup_logger(
        name: str = __name__,
        log_level: int = logging.INFO,
        log_file: str = "/tmp/matrixinfer.log",
        max_bytes: int = 5 * 1024 * 1024,  # 5MB
        backup_count: int = 5
) -> logging.Logger:
    """
    Sets up and returns a logger with stream and rotating file handlers.

    Args:
        name (str): The name of the logger.
        log_level (int): Logging level (e.g., logging.INFO).
        log_file (str): Path to the log file.
        max_bytes (int): Max size in bytes before log is rotated.
        backup_count (int): Number of backup files to keep.

    Returns:
        logging.Logger: Configured logger instance.
    """
    logger = logging.getLogger(name)
    logger.setLevel(log_level)

    if not logger.handlers:
        formatter = logging.Formatter(
            fmt="%(asctime)s - %(levelname)s - [%(filename)s:%(lineno)d] - %(message)s"
        )

        # Console (stream) handler
        stream_handler = logging.StreamHandler(sys.stdout)
        stream_handler.setFormatter(formatter)
        logger.addHandler(stream_handler)

        # Rotating file handler
        try:
            os.makedirs(os.path.dirname(log_file), exist_ok=True)
            file_handler = RotatingFileHandler(
                log_file, maxBytes=max_bytes, backupCount=backup_count
            )
            file_handler.setFormatter(formatter)
            logger.addHandler(file_handler)
        except Exception as e:
            logger.warning(f"Failed to set up rotating file handler: {e}")

    return logger
