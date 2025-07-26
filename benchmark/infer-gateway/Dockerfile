# Use Python 3.12 as base image
FROM python:3.12-slim

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    git \
    curl \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install uv for Python package management
RUN curl -LsSf https://astral.sh/uv/install.sh | sh && \
    echo 'export PATH="/root/.cargo/bin:$PATH"' >> ~/.bashrc && \
    . ~/.bashrc

RUN git clone https://github.com/YaoZengzeng/sglang.git && \
    cd sglang && \
    git fetch && \
    git checkout benchmark --

RUN pip install aiohttp numpy requests tqdm transformers datasets pybase64 huggingface_hub modelscope

# Create a virtual environment and install dependencies
RUN . ~/.bashrc && \
    uv venv && \
    . .venv/bin/activate

RUN wget -P /tmp https://huggingface.co/datasets/anon8231489123/ShareGPT_Vicuna_unfiltered/resolve/main/ShareGPT_V3_unfiltered_cleaned_split.json

ENTRYPOINT ["python3", "sglang/python/sglang/bench_serving.py"]
