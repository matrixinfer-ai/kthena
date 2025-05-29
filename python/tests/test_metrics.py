from pprint import pprint

import pytest

from python.runtime.collect import MetricAdapter
from python.runtime.standard import MetricStandard

# 固定的Prometheus格式字符串
SGLANG_PROMETHEUS_METRICS = """
# HELP sglang:cache_hit_rate The cache hit rate
# TYPE sglang:cache_hit_rate gauge
sglang:cache_hit_rate{model_name="meta-llama/Llama-3.1-8B-Instruct"} 0.007507552643049313
# HELP sglang:time_to_first_token_seconds Histogram of time to first token in seconds.
# TYPE sglang:time_to_first_token_seconds histogram
sglang:time_to_first_token_seconds_sum{model_name="meta-llama/Llama-3.1-8B-Instruct"} 2.3518979474117756e+06
sglang:time_to_first_token_seconds_bucket{le="0.001",model_name="meta-llama/Llama-3.1-8B-Instruct"} 0.0
sglang:time_to_first_token_seconds_bucket{le="0.005",model_name="meta-llama/Llama-3.1-8B-Instruct"} 0.0
sglang:time_to_first_token_seconds_bucket{le="0.08",model_name="meta-llama/Llama-3.1-8B-Instruct"} 6.0
sglang:time_to_first_token_seconds_bucket{le="+Inf",model_name="meta-llama/Llama-3.1-8B-Instruct"} 11008.0
sglang:time_to_first_token_seconds_count{model_name="meta-llama/Llama-3.1-8B-Instruct"} 11008.0
""".strip()


def test_build_operators_dict_with_valid_engine():
    engine_name = "sglang"
    metric_standard1 = MetricStandard(engine_name)
    assert isinstance(metric_standard1.metric_operators_dict, dict)

    engine_name = "sgLANG"
    metric_standard2 = MetricStandard(engine_name)
    assert metric_standard1.metric_operators_dict == metric_standard2.metric_operators_dict


def test_build_operators_dict_with_invalid_engine():
    invalid_engine_name = "invalid_engine"

    with pytest.raises(ValueError, match=f"Unsupported engine : {invalid_engine_name}"):
        MetricStandard(invalid_engine_name)


@pytest.fixture
def mock_metric_standard():
    # Assuming MetricStandard requires an engine parameter
    return MetricStandard(engine="sglang")


def test_metric_adapter_initialization_with_valid_input():
    standard = MetricStandard("sglang")
    adapter = MetricAdapter(SGLANG_PROMETHEUS_METRICS, standard)
    print()
    pprint(adapter.metrics)
    assert len(adapter.metrics) == 3


def test_metric_adapter_initialization_with_invalid_metric_text():
    invalid_metric_text = '''
    # HELP invalid_metric Invalid metric format
    INVALID_TYPE invalid_metric 1
    '''
    standard = MetricStandard("sglang")

    with pytest.raises(ValueError):
        MetricAdapter(invalid_metric_text, standard)


def test_metric_adapter_initialization_with_exception_in_standard():
    class FailingMetricStandard(MetricStandard):
        def process(self, origin_metric):
            raise RuntimeError("Processing error")

    standard = FailingMetricStandard("sglang")

    with pytest.raises(RuntimeError):
        MetricAdapter(SGLANG_PROMETHEUS_METRICS, standard)


def test_metric_adapter_handles_empty_metrics():
    empty_metric_text = ""
    standard = MetricStandard("sglang")

    adapter = MetricAdapter(empty_metric_text, standard)
    assert len(adapter.metrics) == 0
