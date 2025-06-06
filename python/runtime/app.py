import logging
import argparse
from fastapi import FastAPI, APIRouter, Request
from fastapi.responses import Response
import httpx
from starlette.responses import JSONResponse

from env import TIMEOUT, TARGET_SERVICE_URL
import uvicorn
from contextlib import asynccontextmanager

from collect import process_metrics
from standard import MetricStandard

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
    yield
    await app.state.client.aclose()
    return


@router.get("/health")
async def get_health():
    return JSONResponse(content={"status": "ok"}, status_code=200)


@router.get("/metrics")
async def metrics(request: Request):
    state = request.app.state
    response = await state.client.get(TARGET_SERVICE_URL + state.engine_metrics_url)
    response_content = await process_metrics(response.text, state.metric_standard)
    return Response(content=response_content, media_type="text/plain")


def application(args: argparse.Namespace) -> FastAPI:
    app = FastAPI(lifespan=lifespan)
    app.state.metric_standard = MetricStandard(args.engine)
    app.state.engine_metrics_url = args.url
    app.include_router(router)
    return app


def main():
    parser = argparse.ArgumentParser(
        description="Metric Collector",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument(
        "-E",
        "--engine",
        type=str,
        required=True,
        help="Inference engine name",
    )
    parser.add_argument(
        "-H",
        "--host",
        type=str,
        default="0.0.0.0",
        help="Host address",
    )
    parser.add_argument(
        "-P",
        "--port",
        type=int,
        default=8000,
        help="Port number",
    )
    parser.add_argument(
        "-U",
        "--url",
        type=str,
        default="/metrics",
        help="Metrics url",
    )
    args = parser.parse_args()
    app = application(args)
    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
