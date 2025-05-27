import os

TARGET_SERVICE_URL = os.getenv("TARGET_SERVICE_URL", "http://target-pod:8000")
TIMEOUT = float(os.getenv("REQUEST_TIMEOUT", "30.0"))