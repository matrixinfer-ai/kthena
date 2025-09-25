# Copyright The Volcano Authors
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
import types
from types import SimpleNamespace

from fastapi.testclient import TestClient

from kthena.runtime.app import create_application, get_app_state


class _FakeAsyncResponse:
    def __init__(self, status_code: int = 200, payload: dict | None = None, text: str = "OK"):
        self.status_code = status_code
        self._payload = payload or {"status": "ok"}
        self.text = text

    def json(self):
        return self._payload


class _FakeAsyncClient:
    def __init__(self, *args, **kwargs):
        pass

    async def post(self, url: str, json: dict | None = None):
        return _FakeAsyncResponse(status_code=200, payload={"url": url, "json": json})

    async def aclose(self):
        return None


class _FakeSyncResponse:
    def __init__(self, status_code: int = 200, payload: dict | None = None, text: str = "OK"):
        self.status_code = status_code
        self._payload = payload or {"status": "ok"}
        self.text = text

    def json(self):
        return self._payload


class _FakeSyncClient:
    def __init__(self, *args, **kwargs):
        pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def post(self, url: str, json: dict | None = None):
        # Simulate engine endpoint success
        return _FakeSyncResponse(status_code=200, payload={"url": url, "json": json})


def _make_args():
    return SimpleNamespace(
        engine="vllm",
        host="127.0.0.1",
        port=0,
        engine_base_url="http://engine.local:8000",
        engine_metrics_path="/metrics",
        pod="pod-1.ns",
        model="test-model",
    )


def test_load_lora_adapter_success(monkeypatch):
    # Arrange: create app and inject fake async client and downloader
    app = create_application(_make_args())

    state = get_app_state(app)
    monkeypatch.setattr("httpx.AsyncClient", _FakeAsyncClient)
    state.client = _FakeAsyncClient()

    monkeypatch.setattr("kthena.runtime.app.download_model", lambda *args, **kwargs: None)
    monkeypatch.setattr("httpx.Client", _FakeSyncClient)

    with TestClient(app) as client:
        body = {
            "lora_name": "lora-sql",
            "source": "s3://bucket/lora-sql",
            "output_dir": "/tmp/lora",
            "config": {},
            "max_workers": 1,
            "async_download": False,
        }

        # Act
        resp = client.post("/v1/load_lora_adapter", json=body)

        # Assert
        assert resp.status_code == 200
        data = resp.json()
        assert data


def test_unload_lora_adapter_success(monkeypatch):
    # Arrange: create app and inject fake async client
    app = create_application(_make_args())

    state = get_app_state(app)
    monkeypatch.setattr("httpx.AsyncClient", _FakeAsyncClient)
    state.client = _FakeAsyncClient()

    with TestClient(app) as client:
        body = {"lora_name": "lora-sql"}

        # Act
        resp = client.post("/v1/unload_lora_adapter", json=body)

        # Assert
        assert resp.status_code == 200
        data = resp.json()
        assert data


def test_load_lora_adapter_async_download_success(monkeypatch):
    # Arrange: create app, inject clients, and patch download + sync client
    app = create_application(_make_args())
    state = get_app_state(app)
    monkeypatch.setattr("httpx.AsyncClient", _FakeAsyncClient)
    state.client = _FakeAsyncClient()

    monkeypatch.setattr("kthena.runtime.app.download_model", lambda *args, **kwargs: None)
    monkeypatch.setattr("httpx.Client", _FakeSyncClient)

    with TestClient(app) as client:
        body = {
            "lora_name": "lora-sql",
            "source": "s3://bucket/lora-sql",
            "output_dir": "/tmp/lora",
            "config": {},
            "max_workers": 1,
            "async_download": True,
        }

        # Act
        resp = client.post("/v1/load_lora_adapter", json=body)

        # Assert (background task returns 202 Accepted)
        assert resp.status_code == 202
