from prometheus_client.core import Sample
from typing import Dict
from abc import ABC, abstractmethod


class LabelOperator(ABC):
    def __init__(self, target_label: str):
        self.target_label = target_label
        pass

    def register_name(self) -> str:
        return self.target_label

    @abstractmethod
    def process(self) -> str:
        pass


class RenameLabel(LabelOperator):

    def __init__(self, target_label: str, rename_label: str):
        super().__init__(target_label)
        self.rename_label = rename_label

    def process(self) -> str:
        return self.rename_label


class RemoveLabel(LabelOperator):

    def process(self):
        return None
