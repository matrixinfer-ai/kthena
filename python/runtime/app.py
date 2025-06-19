import logging
import argparse
import os
from fastapi import FastAPI, APIRouter, Request, HTTPException
from fastapi.responses import Response
import httpx
from starlette.responses import JSONResponse
from contextlib import asynccontextmanager

import uvicorn
from runtime.collect import process_metrics
from runtime.standard import MetricStandard

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
    app.state.engine_metrics_url = args.url
    
    app.include_router(router)
    
    logger.info(f"Application configured for engine: {args.engine}")
    logger.info(f"Metrics URL: {args.url}")
    
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
        "-U", "--url",
        type=str,
        default="http://localhost:8000/metrics",
        help="URL endpoint to fetch metrics from engine"
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


if __name__ == "__main__":
    main()