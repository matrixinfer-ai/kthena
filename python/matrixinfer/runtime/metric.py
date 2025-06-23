from prometheus_client.core import Metric, Sample
from abc import ABC, abstractmethod

class MetricOperator(ABC):
    def __init__(
        self,
        target_metric_name: str,
            label_operators_map=None,
    ):
        if label_operators_map is None:
            label_operators_map = {}
        self.target_metric_name = target_metric_name
        self.label_operators_map = label_operators_map
        pass

    def register_name(self) -> str:
        return self.target_metric_name

    def _process_labels(
            self,
            sample: Sample,
    ) -> dict[str, str]:
        labels = {}
        if len(self.label_operators_map) != 0:
            for k, v in sample.labels.items():
                if k in self.label_operators_map:
                    nk = self.label_operators_map[k].process()
                    if nk:
                        labels[nk] = v
                else:
                    labels[k] = v
        else:
            labels = sample.labels
        return labels

    @abstractmethod
    def process(self, origin_metric: Metric) -> Metric:
        pass


class RenameMetric(MetricOperator):
    def __init__(
            self,
            target_metric_name: str,
            rename_metric_name: str,
            label_operators=None,
    ):
        if label_operators is None:
            label_operators = []
        super().__init__(
            target_metric_name, {op.register_name(): op for op in label_operators}
        )
        self.rename_metric_name = rename_metric_name

    def register_name(self) -> str:
        return super().register_name()

    def _process_sample(
            self,
            sample: Sample,
    ) -> Sample:
        labels = self._process_labels(sample)
        new_sample_name = self.rename_metric_name + sample.name[len(self.target_metric_name):]
        return Sample(
            new_sample_name,
            labels,
            sample.value,
            sample.timestamp,
            sample.exemplar,
        )

    def process(self, origin_metric: Metric) -> Metric:
        metric_family = Metric(
            self.rename_metric_name,
            origin_metric.documentation,
            origin_metric.type,
            origin_metric.unit,
        )
        for sample in origin_metric.samples:
            new_sample = self._process_sample(sample)
            metric_family.samples.append(new_sample)
        return metric_family
