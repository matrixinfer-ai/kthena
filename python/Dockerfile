FROM python:3.9.6-slim
RUN apt-get update && apt-get install -y rsync && apt-get clean && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY downloader/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY downloader/ .
ENTRYPOINT ["python", "app.py"]