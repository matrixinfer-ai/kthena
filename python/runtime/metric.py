from prometheus_client.core import Metric, Sample
from typing import Dict, List, Optional
from abc import ABC, abstractmethod

from python.runtime.label import LabelOperator


class MetricOperator(ABC):
    def __init__(
        self,
        target_metric_name: str,
        label_operators_map: Dict[str, LabelOperator] = {},
    ):
        self.target_metric_name = target_metric_name
        self.label_operators_map = label_operators_map
        pass

    def register_name(self) -> str:
        return self.target_metric_name

    def _process_sample(
        self,
        sample: Sample,
    ) -> Sample:
        labels = {}
        if len(self.label_operators_map) != 0:
            for k, v in sample.labels.items():
                if k in self.label_operators_map:
                    nk = self.label_operators_map[k].process(k)
                    if not nk:
                        labels[nk] = v
                else:
                    labels[k] = v
        else:
            labels = sample.labels
        Sample(
            sample.name,
            labels,
            sample.value,
            sample.timestamp,
            sample.exemplar,
            sample.native_histogram,
        )

    @abstractmethod
    def process(self, origin_metric: Metric) -> str:
        pass


class RenameMetric(MetricOperator):
    def __init__(
        self,
        target_metric_name: str,
        rename_metric_name: str,
        label_operators: List[LabelOperator] = [],
    ):
        super().__init__(
            target_metric_name, {op.register_name(): op for op in label_operators}
        )
        self.rename_metric_name = rename_metric_name

    def register_name(self) -> str:
        return super().register_name()

    def process(self, origin_metric: Metric) -> str:
        metric_family = Metric(
            self.rename_metric_name,
            origin_metric.documentation,
            origin_metric.type,
            origin_metric.unit,
        )
        for sample in origin_metric.samples:
            new_sample = self._process_sample(sample)
            new_sample.name = (
                self.rename_metric_name + sample.name[len(self.target_metric_name) :]
            )
            metric_family.add_sample(new_sample)


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
