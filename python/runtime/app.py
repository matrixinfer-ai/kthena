import logging
import argparse
from fastapi import FastAPI, APIRouter, Request
from fastapi.responses import Response
import httpx
from prometheus_client import generate_latest, CollectorRegistry
from prometheus_client.parser import text_string_to_metric_families
from os import TIMEOUT, TARGET_SERVICE_URL
import uvicorn
from contextlib import asynccontextmanager

from python.runtime.standard import MetricStandard

logger = logging.getLogger(__name__)
router = APIRouter()


@asynccontextmanager
async def lifespan(app: FastAPI):
    app.state.client = httpx.AsyncClient(
        limits=httpx.Limits(
            max_keepalive_connections=200,
            keepalive_expiry=60,
        ),
        timeout=httpx.Timeout(connect=10.0),
    )
    yield
    await app.state.client.aclose()
    return


@router.get("/metrics")
async def metrics(request: Request):
    state = request.app.state
    response = await state.client.get(TARGET_SERVICE_URL + "/metrics")
    response_content = await _process_metrics(response.text, state.metric_standard)
    return Response(content=response_content, media_type="text/plain")


async def _process_metrics(origin_metric_text, standard: MetricStandard) -> bytes:
    registry = CollectorRegistry()
    for origin_metric in text_string_to_metric_families(origin_metric_text):
        registry.register(origin_metric)
        processed_metrics = standard.process(origin_metric)
        registry.register(processed_metrics)
    return generate_latest(registry)


async def application(args: argparse.Namespace) -> FastAPI:
    app = FastAPI(lifespan=lifespan)
    app.state.metric_standard = MetricStandard(args.engine)
    app.include_router(router)
    return app


def main():
    parser = argparse.ArgumentParser(
        description="Metric Collector",
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )
    parser.add_argument(
        "-e",
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
        "-p",
        "--port",
        type=int,
        default=8000,
        help="Port number",
    )
    args = parser.parse_args()
    app = application(args)
    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
