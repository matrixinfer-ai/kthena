#!/usr/bin/env bash

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

set -eo pipefail

ROOT_DIR="$(git rev-parse --show-toplevel)"

GO_FILES=$(find "$ROOT_DIR" -not -path "/vendor/*" -type f -name '*.go')
PY_FILES=$(find "$ROOT_DIR" -not -path "/venv/*" -type f -name '*.py')

GO_TPL="$ROOT_DIR/hack/boilerplate.go.txt"
PY_TPL="$ROOT_DIR/hack/boilerplate.py.txt"

for file in $GO_FILES; do
  if ! grep -q "Copyright MatrixInfer-AI Authors" "$file"; then
    (cat "$GO_TPL" && echo && cat "$file") | sponge "$file"
  fi
done

for file in $PY_FILES; do
  if ! grep -q "Copyright MatrixInfer-AI Authors" "$file"; then
    (cat "$PY_TPL" && echo && cat "$file") | sponge "$file"
  fi
done

echo "Update Copyright Done"