import logging
from typing import Iterable, List

from prometheus_client import generate_latest
from prometheus_client.parser import text_string_to_metric_families
from prometheus_client.core import Metric
from prometheus_client.registry import Collector, CollectorRegistry

from runtime.standard import MetricStandard

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class MetricAdapter(Collector):
    def __init__(self, origin_metric_text: str, standard: MetricStandard):
        self.metrics: List[Metric] = []
        try:
            for origin_metric in text_string_to_metric_families(origin_metric_text):
                self.metrics.append(origin_metric)
                processed = standard.process(origin_metric)
                if processed is not None:
                    self.metrics.append(processed)
        except ValueError as ve:
            logger.error(f"Invalid metric text: {str(ve)}")
            raise ValueError(f"Failed to parse metric text: {str(ve)}")
        except Exception as e:
            logger.error(f"Error processing metrics in MetricAdapter: {str(e)}")
            raise RuntimeError(f"Failed to initialize MetricAdapter: {str(e)}")

    def collect(self) -> Iterable[Metric]:
        for metric in self.metrics:
            yield metric


async def process_metrics(origin_metric_text: str, standard: MetricStandard) -> bytes:
    registry = CollectorRegistry()
    try:
        if not origin_metric_text.strip():
            logger.warning("Empty metric text provided.")
            return generate_latest(registry)
        registry.register(MetricAdapter(origin_metric_text, standard))
        return generate_latest(registry)
    except ValueError as ve:
        logger.error(f"Invalid metric text: {str(ve)}")
        raise
    except Exception as e:
        logger.error(f"Unexpected error in process_metrics: {str(e)}")
        raise RuntimeError(f"Failed to process metrics: {str(e)}")
