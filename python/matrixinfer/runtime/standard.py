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

from typing import Dict, Optional, List
from enum import Enum

from prometheus_client import Metric
from matrixinfer.runtime.metric import MetricOperator, RenameMetric


class SupportedEngine(Enum):
    VLLM = "vllm"
    SGLANG = "sglang"


class StandardMetricNames:
    GENERATION_TOKENS_TOTAL = "matrixinfer:generation_tokens_total"
    NUM_REQUESTS_WAITING = "matrixinfer:num_requests_waiting"
    TIME_TO_FIRST_TOKEN_SECONDS = "matrixinfer:time_to_first_token_seconds"
    TIME_PER_OUTPUT_TOKEN_SECONDS = "matrixinfer:time_per_output_token_seconds"
    E2E_REQUEST_LATENCY_SECONDS = "matrixinfer:e2e_request_latency_seconds"


STANDARD_RULES: Dict[str, List[MetricOperator]] = {
    SupportedEngine.VLLM.value: [
        RenameMetric(
            "vllm:generation_tokens_total",
            StandardMetricNames.GENERATION_TOKENS_TOTAL,
        ),
        RenameMetric(
            "vllm:num_requests_waiting", 
            StandardMetricNames.NUM_REQUESTS_WAITING
        ),
        RenameMetric(
            "vllm:time_to_first_token_seconds",
            StandardMetricNames.TIME_TO_FIRST_TOKEN_SECONDS,
        ),
        RenameMetric(
            "vllm:time_per_output_token_seconds",
            StandardMetricNames.TIME_PER_OUTPUT_TOKEN_SECONDS,
        ),
        RenameMetric(
            "vllm:e2e_request_latency_seconds",
            StandardMetricNames.E2E_REQUEST_LATENCY_SECONDS,
        ),
    ],
    SupportedEngine.SGLANG.value: [
        RenameMetric(
            "sglang:generation_tokens_total",
            StandardMetricNames.GENERATION_TOKENS_TOTAL,
        ),
        RenameMetric(
            "sglang:num_queue_reqs", 
            StandardMetricNames.NUM_REQUESTS_WAITING
        ),
        RenameMetric(
            "sglang:time_to_first_token_seconds",
            StandardMetricNames.TIME_TO_FIRST_TOKEN_SECONDS,
        ),
        RenameMetric(
            "sglang:time_per_output_token_seconds",
            StandardMetricNames.TIME_PER_OUTPUT_TOKEN_SECONDS,
        ),
        RenameMetric(
            "sglang:e2e_request_latency_seconds",
            StandardMetricNames.E2E_REQUEST_LATENCY_SECONDS,
        ),
    ],
}


class UnsupportedEngineError(ValueError):
    pass


class MetricStandard:
    
    def __init__(self, engine: str):
        self.engine = engine.lower()
        self.metric_operators_dict = self._build_operators_dict()

    def _build_operators_dict(self) -> Dict[str, MetricOperator]:
        if self.engine not in STANDARD_RULES:
            supported_engines = list(STANDARD_RULES.keys())
            raise UnsupportedEngineError(
                f"Unsupported engine: {self.engine}. "
                f"Supported engine: {', '.join(supported_engines)}"
            )
        
        return {
            operator.register_name(): operator
            for operator in STANDARD_RULES[self.engine]
        }

    def process(self, origin_metric: Metric) -> Optional[Metric]:
        if not self.metric_operators_dict:
            return None
            
        operator = self.metric_operators_dict.get(origin_metric.name)
        if operator is None:
            return None
            
        return operator.process(origin_metric)

    def is_supported_metric(self, metric_name: str) -> bool:
        return metric_name in self.metric_operators_dict

    def get_supported_metrics(self) -> List[str]:
        return list(self.metric_operators_dict.keys())