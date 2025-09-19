# Copyright The Volcano Authors.
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
