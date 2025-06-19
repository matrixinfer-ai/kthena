FROM python:3.9-slim AS downloader

RUN apt-get update && \
    apt-get install -y curl unzip rsync && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN set -ex && \
    ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then \
      curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"; \
    elif [ "$ARCH" = "aarch64" ]; then \
      curl "https://awscli.amazonaws.com/awscli-exe-linux-aarch64.zip" -o "awscliv2.zip"; \
    else \
      echo "Unsupported architecture: $ARCH" && exit 1; \
    fi && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip aws

WORKDIR /app
COPY downloader/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY downloader/ .
ENTRYPOINT ["python", "app.py"]

FROM python:3.9.6-slim AS runtime
WORKDIR /app
COPY runtime/ ./runtime/
RUN pip install --no-cache-dir -r runtime/requirements.txt
ENV PYTHONPATH="/app"
ENTRYPOINT ["python", "runtime/app.py"]