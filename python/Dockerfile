FROM python:3.9.6-slim AS builder
WORKDIR /app
COPY downloader/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

FROM python:3.9.6-slim
WORKDIR /app
COPY --from=builder /usr/local/lib/python3.9/site-packages /usr/local/lib/python3.9/site-packages
COPY --from=builder /usr/local/bin /usr/local/bin
COPY downloader/ .
ENTRYPOINT ["python", "app.py"]