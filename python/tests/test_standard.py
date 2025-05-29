import pytest
from python.runtime.standard import MetricStandard, STANDARD_RULES


def test_build_operators_dict_with_valid_engine():
    engine_name = "vLLM"

    metric_standard1 = MetricStandard(engine_name)
    assert isinstance(metric_standard1.metric_operators_dict, dict)

    engine_name = "vllm"
    metric_standard2 = MetricStandard(engine_name)
    assert metric_standard1.metric_operators_dict == metric_standard2.m


def test_build_operators_dict_with_invalid_engine():
    invalid_engine_name = "invalid_engine"

    with pytest.raises(ValueError, match=f"Unsupported engine : {invalid_engine_name}"):
        MetricStandard(invalid_engine_name)
