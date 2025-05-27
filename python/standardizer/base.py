import requests
from abc import ABC, abstractmethod

# 抽象基类：框架适配器接口
class FrameworkAdapter(ABC):
    def __init__(self):
        pass

    @abstractmethod
    def standardize_metrics(self, raw_metrics: str, standard_metrics: set) -> str:
        """将原始指标中属于标准化集合的指标进行前缀替换"""
        pass

# SGLang 适配器
class SglangAdapter(FrameworkAdapter):
    def standardize_metrics(self, raw_metrics: str, standard_metrics: set) -> str:
        """仅对标准化指标集合中的指标将 sglang: 替换为 matrixinfer:"""
        lines = raw_metrics.splitlines()
        standardized_lines = []
        for line in lines:
            # 跳过注释和空行
            if line.startswith("#") or not line.strip():
                standardized_lines.append(line)
                continue
            # 检查指标是否在标准化集合中
            metric_name = line.split()[0].split("{")[0]  # 提取指标名称（忽略标签）
            metric_base = metric_name.split(":", 1)[1] if ":" in metric_name else metric_name
            if metric_base in standard_metrics:
                standardized_lines.append(line.replace("sglang:", "matrixinfer:"))
            else:
                standardized_lines.append(line)
        return "\n".join(standardized_lines)

# vLLM 适配器
class VllmAdapter(FrameworkAdapter):
    def standardize_metrics(self, raw_metrics: str, standard_metrics: set) -> str:
        """仅对标准化指标集合中的指标将 vllm: 替换为 matrixinfer:"""
        lines = raw_metrics.splitlines()
        standardized_lines = []
        for line in lines:
            if line.startswith("#") or not line.strip():
                standardized_lines.append(line)
                continue
            metric_name = line.split()[0].split("{")[0]
            metric_base = metric_name.split(":", 1)[1] if ":" in metric_name else metric_name
            if metric_base in standard_metrics:
                standardized_lines.append(line.replace("vllm:", "matrixinfer:"))
            else:
                standardized_lines.append(line)
        return "\n".join(standardized_lines)

# 标准化指标核心类
class MetricsStandardizer:
    def __init__(self, framework: str, standard_metrics: set):
        """初始化标准化器，指定框架和标准化指标集合"""
        self.framework = framework.lower()
        self.standard_metrics = standard_metrics
        self.adapter = self._create_adapter()

    def _create_adapter(self) -> FrameworkAdapter:
        """根据框架类型创建适配器实例"""
        if self.framework == "sglang":
            return SglangAdapter()
        elif self.framework == "vllm":
            return VllmAdapter()
        else:
            raise ValueError(f"Unsupported framework: {self.framework}")

    def standardize_metrics(self) -> str:
        """从 Engine 容器获取指标并标准化"""
        metrics_url = "http://localhost:8000/metrics"
        try:
            response = requests.get(metrics_url, timeout=5)
            response.raise_for_status()
            raw_metrics = response.text
            return self.adapter.standardize_metrics(raw_metrics, self.standard_metrics)
        except requests.RequestException as e:
            raise RuntimeError(f"Failed to fetch metrics: {e}")