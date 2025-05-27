from typing import Dict, Optional

from prometheus_client import Metric
from python.runtime.metric import MetricOperator, RenameMetric


STANDARD_RULES: Dict[str, MetricOperator] = {
    "vllm": [
        RenameMetric(
            "vllm:generation_tokens_total",
            "matrixinfer:generation_tokens_total",
        ),
        RenameMetric("vllm:num_requests_waiting", "matrixinfer:num_requests_waiting"),
        RenameMetric(
            "vllm:time_to_first_token_seconds",
            "matrixinfer:time_to_first_token_seconds",
        ),
        RenameMetric(
            "vllm:time_per_output_token_seconds",
            "matrixinfer:time_per_output_token_seconds",
        ),
        RenameMetric(
            "vllm:e2e_request_latency_seconds",
            "matrixinfer:e2e_request_latency_seconds",
        ),
    ],
    "sglang": [
        RenameMetric(
            "sglang:generation_tokens_total",
            "matrixinfer:generation_tokens_total",
        ),
        RenameMetric("sglang:num_queue_reqs", "matrixinfer:num_requests_waiting"),
        RenameMetric(
            "sglang:time_to_first_token_seconds",
            "matrixinfer:time_to_first_token_seconds",
        ),
        RenameMetric(
            "sglang:time_per_output_token_seconds",
            "matrixinfer:time_per_output_token_seconds",
        ),
        RenameMetric(
            "sglang:e2e_request_latency_seconds",
            "matrixinfer:e2e_request_latency_seconds",
        ),
    ],
}


class MetricStandard:
    def __init__(self, engine: str):
        if engine.lower not in STANDARD_RULES:
            raise ValueError(f"Unsupported engine: {engine}")

        self.metric_operators_dict = {
            op.register(): op for op in STANDARD_RULES[engine.lower]
        }

    def process(self, origin_metric: Metric) -> Optional[Metric]:
        if (
            len(self.metric_operators) == 0
            or origin_metric.name not in self.metric_operators_dict
        ):
            return None
        return self.metric_operators_dict[origin_metric.name].process(origin_metric)
