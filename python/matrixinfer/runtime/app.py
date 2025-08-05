# Copyright MatrixInfer-AI Authors.
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
import argparse
import os
from fastapi import FastAPI, APIRouter, Request, HTTPException
from fastapi.responses import Response
import httpx
from starlette.responses import JSONResponse
from contextlib import asynccontextmanager

import uvicorn

from matrixinfer.runtime.collect import process_metrics
from matrixinfer.runtime.standard import MetricStandard
from fastapi import BackgroundTasks
from matrixinfer.downloader.app import load_config
from matrixinfer.downloader.downloader import download_model

TIMEOUT = float(os.getenv("REQUEST_TIMEOUT", "30.0"))

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

router = APIRouter()


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.client = httpx.AsyncClient(
        limits=httpx.Limits(
            max_keepalive_connections=200,
            keepalive_expiry=60,
        ),
        timeout=httpx.Timeout(TIMEOUT),
    )
    logger.info("HTTP client initialized")

    yield

    await app.state.client.aclose()
    logger.info("HTTP client closed")


@router.get("/health", tags=["Health"])
async def health_check() -> JSONResponse:
    return JSONResponse(
        content={"status": "healthy", "service": "runtime"},
        status_code=200
    )


@router.get("/metrics", tags=["Metrics"])
async def get_metrics(request: Request) -> Response:
    try:
        state = request.app.state

        response = await state.client.get(state.engine_metrics_url)
        response.raise_for_status()

        processed_content = await process_metrics(
            response.text,
            state.metric_standard
        )

        return Response(
            content=processed_content,
            media_type="text/plain; charset=utf-8"
        )

    except httpx.HTTPError as e:
        logger.error(f"Failed to fetch metrics: {e}")
        raise HTTPException(
            status_code=502,
            detail=f"Failed to fetch metrics from engine: {str(e)}"
        )
    except Exception as e:
        logger.error(f"Error processing metrics: {e}")
        raise HTTPException(
            status_code=500,
            detail=f"Error processing metrics: {str(e)}"
        )


def create_application(args: argparse.Namespace) -> FastAPI:
    app = FastAPI(lifespan=lifespan)

    app.state.metric_standard = MetricStandard(args.engine)
    app.state.engine_base_url = args.engine_base_url
    app.state.engine_metrics_url = args.engine_base_url + args.engine_metrics_path

    app.include_router(router)

    logger.info(f"Application configured for engine: {args.engine}")
    logger.info(f"Engine base URL: {args.engine_base_url}")
    logger.info(f"Engine metrics URL: {args.engine_base_url + args.engine_metrics_path}")

    return app


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Metric Collector Service",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )

    parser.add_argument(
        "-E", "--engine",
        type=str,
        required=True,
        help="Inference engine name"
    )

    parser.add_argument(
        "-H", "--host",
        type=str,
        default="0.0.0.0",
        help="Host address to bind the server"
    )

    parser.add_argument(
        "-P", "--port",
        type=int,
        default=9000,
        help="Port number to bind the server"
    )

    parser.add_argument(
        "-B", "--engine-base-url",
        type=str,
        default="http://localhost:8000",
        help="Base URL of the engine server"
    )

    parser.add_argument(
        "-M", "--engine-metrics-path",
        type=str,
        default="/metrics",
        help="Metrics endpoint path"
    )

    return parser.parse_args()


def main() -> None:
    try:
        args = parse_arguments()
        app = create_application(args)

        logger.info(f"Starting service on {args.host}:{args.port}")

        uvicorn.run(
            app,
            host=args.host,
            port=args.port,
            log_level="info"
        )

    except KeyboardInterrupt:
        logger.info("Service stopped by user")
    except Exception as e:
        logger.error(f"Failed to start service: {e}")
        raise


@router.post("/v1/load_lora_adapter", tags=["LoRA"])
async def load_lora_adapter(request: Request, background_tasks: BackgroundTasks) -> JSONResponse:
    """
    Load LoRA adapter with integrated download capability.
    
    Request body should contain:
    - lora_name: str - Name of the LoRA adapter
    - source: str (optional) - Source URL for downloading (s3://, obs://, pvc://, or HuggingFace)
    - output_dir: str (optional) - Directory to download the adapter to
    - config: dict (optional) - Download configuration (access_key, secret_key, endpoint, hf_token, etc.)
    - max_workers: int (optional) - Number of parallel workers for download (default: 8)
    - async_download: bool (optional) - Whether to run download in background (default: False)
    """
    try:
        state = request.app.state
        body = await request.json()
        # Extract parameters
        lora_name = body.get("lora_name")
        lora_path = body.get("output_dir")
        source = body.get("source")
        output_dir = body.get("output_dir")
        config = load_config(body.get("config",{}))
        max_workers = body.get("max_workers", 8)
        async_download = body.get("async_download", False)

        def download_and_load_task():
            try:
                logger.info(f"Downloading LoRA adapter from {source} to {output_dir}")
                download_model(source, output_dir, config, max_workers)
                logger.info(f"LoRA adapter download completed: {source} -> {output_dir}")

                # Load the adapter
                load_body = {
                    "lora_name": lora_name,
                    "lora_path": lora_path
                }

                # Make synchronous request to engine
                with httpx.Client(timeout=httpx.Timeout(TIMEOUT)) as sync_client:
                    response = sync_client.post(f"{state.engine_base_url}/v1/load_lora_adapter", json=load_body)
                    if response.status_code >= 400:
                        error_detail = f"HTTP {response.status_code}: {response.text}"
                        logger.error(f"Engine request failed: {error_detail}")
                        raise Exception(error_detail)
                    logger.info(f"LoRA adapter loaded successfully: {lora_name}")
                    return response.json()

            except Exception as e:
                logger.error(f"Error in download and load task: {e}")
                raise

        if async_download:
            # Run download and load in background
            background_tasks.add_task(download_and_load_task)
            return JSONResponse(
                content={
                    "status": "started",
                    "message": "LoRA adapter download and load started in background",
                    "lora_name": lora_name,
                    "source": source,
                    "output_dir": output_dir
                },
                status_code=202
            )
        else:
            # Run synchronously
            logger.info(f"Downloading LoRA adapter from {source} to {output_dir}")
            download_model(source, output_dir, config, max_workers)
            logger.info(f"LoRA adapter download completed: {source} -> {output_dir}")

            # Load the adapter
            load_body = {
                "lora_name": lora_name,
                "lora_path": lora_path
            }

            response = await state.client.post(f"{state.engine_base_url}/v1/load_lora_adapter", json=load_body)
            if response.status_code >= 400:
                error_detail = f"HTTP {response.status_code}: {response.text}"
                logger.error(f"Engine request failed: {error_detail}")
                raise HTTPException(status_code=response.status_code, detail=error_detail)
            return JSONResponse(content=response.text, status_code=response.status_code)

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error loading LoRA adapter: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@router.post("/v1/unload_lora_adapter", tags=["LoRA"])
async def unload_lora_adapter(request: Request) -> JSONResponse:
    try:
        state = request.app.state
        body = await request.json()
        response = await state.client.post(f"{state.engine_base_url}/v1/unload_lora_adapter", json=body)
        if response.status_code >= 400:
            error_detail = f"HTTP {response.status_code}: {response.text}"
            logger.error(f"Engine request failed: {error_detail}")
            raise HTTPException(status_code=response.status_code, detail=error_detail)
        return JSONResponse(content=response.text, status_code=response.status_code)
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error unloading LoRA adapter: {e}")
        raise HTTPException(status_code=500, detail=str(e))

@router.post("/v1/download_model", tags=["Download"])
async def download_model_endpoint(request: Request, background_tasks: BackgroundTasks) -> JSONResponse:
    """
    Download model from various sources (S3, OBS, PVC, HuggingFace)
    
    Request body should contain:
    - source: str - Model source URL (s3://, obs://, pvc://, or HuggingFace model name)
    - output_dir: str - Local directory to save the model
    - config: dict - Configuration for the downloader (access_key, secret_key, endpoint, hf_token, etc.)
    - max_workers: int (optional) - Number of parallel workers for download (default: 8)
    - async_download: bool (optional) - Whether to run download in background (default: False)
    """
    try:
        body = await request.json()

        # Validate required parameters
        source = body.get("source")
        output_dir = body.get("output_dir")
        config = load_config(body.get("config",{}))
        max_workers = body.get("max_workers", 8)
        async_download = body.get("async_download", False)

        if not source:
            raise HTTPException(status_code=400, detail="Missing required parameter: source")
        if not output_dir:
            raise HTTPException(status_code=400, detail="Missing required parameter: output_dir")

        logger.info(f"Starting model download from {source} to {output_dir}")

        def download_task():
            try:
                download_model(source, output_dir, config, max_workers)
                logger.info(f"Model download completed successfully: {source} -> {output_dir}")
            except Exception as e:
                logger.error(f"Model download failed: {e}")
                raise

        if async_download:
            # Run download in background
            background_tasks.add_task(download_task)
            return JSONResponse(
                content={
                    "status": "started",
                    "message": "Model download started in background",
                    "source": source,
                    "output_dir": output_dir
                },
                status_code=202
            )
        else:
            # Run download synchronously
            download_task()
            return JSONResponse(
                content={
                    "status": "completed",
                    "message": "Model download completed successfully",
                    "source": source,
                    "output_dir": output_dir
                },
                status_code=200
            )

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error in download endpoint: {e}")
        raise HTTPException(status_code=500, detail=f"Download failed: {str(e)}")

if __name__ == "__main__":
    main()