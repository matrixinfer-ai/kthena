import logging
from typing import Iterable, List

from prometheus_client import generate_latest
from prometheus_client.parser import text_string_to_metric_families
from prometheus_client.core import Metric
from prometheus_client.registry import Collector, CollectorRegistry

from matrixinfer.runtime.standard import MetricStandard

logger = logging.getLogger(__name__)


class MetricAdapter(Collector):
    def __init__(self, origin_metric_text: str, standard: MetricStandard):
        self.metrics: List[Metric] = self._parse_and_process_metrics(origin_metric_text, standard)

    def _parse_and_process_metrics(self, origin_metric_text: str, standard: MetricStandard) -> List[Metric]:
        metrics = []
        
        if not origin_metric_text.strip():
            logger.warning("Empty metric text provided")
            return metrics
            
        try:
            for origin_metric in text_string_to_metric_families(origin_metric_text):
                metrics.append(origin_metric)
                
                processed_metric = standard.process(origin_metric)
                if processed_metric is not None:
                    metrics.append(processed_metric)
                    
        except ValueError as e:
            logger.error(f"Invalid metric text format: {e}")
            raise ValueError(f"Failed to parse metric text: {e}")
        except Exception as e:
            logger.error(f"Unexpected error processing metrics: {e}")
            raise RuntimeError(f"Failed to initialize MetricAdapter: {e}")
            
        return metrics

    def collect(self) -> Iterable[Metric]:
        yield from self.metrics


async def process_metrics(origin_metric_text: str, standard: MetricStandard) -> bytes:
    if not isinstance(origin_metric_text, str):
        raise TypeError("Metric text must be a string")
    
    if not isinstance(standard, MetricStandard):
        raise TypeError("Standard must be a MetricStandard instance")
    
    registry = CollectorRegistry()
    
    try:
        adapter = MetricAdapter(origin_metric_text, standard)
        registry.register(adapter)
        return generate_latest(registry)
        
    except (ValueError, RuntimeError):
        raise
    except Exception as e:
        logger.error(f"Unexpected error in process_metrics: {e}")
        raise RuntimeError(f"Failed to process metrics: {e}")