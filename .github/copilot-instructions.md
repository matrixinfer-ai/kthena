# Copilot Instructions for MatrixInfer

This is a Kubernetes‑native LLM inference platform. Follow these repo‑specific patterns to stay productive and compatible with existing components.

## Architecture & Key Paths
- Control vs Data Plane: controllers manage CRDs; Infer Gateway routes traffic to pods.
- Gateway entry: `cmd/infer-gateway/` (Gin server; flags `--port`, `--tls-cert`, `--tls-key`).
- Gateway core: `pkg/infer-gateway/` → `router/` (OpenAI `/v1/*`, SSE), `scheduler/` (plugin framework), `filters/` (JWT, rate limit), `connectors/` (PD KV connectors), `controller/` (informers), `datastore/` (state).
- CRDs: `pkg/apis/networking/v1alpha1/` → `ModelServer` (engine, `workloadSelector`, `workloadPort`, `trafficPolicy`, `kvConnector`), `ModelRoute` (rules, rateLimit). Examples under `examples/infer-gateway/`.
- Gateway config: `/etc/config/gatewayConfiguration.yaml` (scheduler plugins + JWT `issuer|audiences|jwksUri`).

## Workflows
- Codegen & clients: edit CRDs → `make generate` (deepcopy, client-go, Helm CRDs in `charts/**/crds`).
- Docs: `make gen-docs` (CRD refs to `docs/matrixinfer/docs/reference/crd` + `minfer` CLI docs).
- Build: `make build` (binaries in `bin/`: infer-gateway, controllers, webhooks, autoscaler, `minfer`).
- Test/Lint: `make test` (envtest, skips e2e/client-go), `make lint` / `make lint-fix`.
- Images: `make docker-build-...` per component; push all with `make docker-push` (`HUB`/`TAG`).

## Conventions
- Logging: `k8s.io/klog/v2` (Gin logger enabled). Avoid direct `fmt.Printf` in server paths.
- Informers-first: extend `pkg/infer-gateway/controller` + `datastore` instead of API calls in hot request paths.
- Scheduling plugins: implement `framework.FilterPlugin` / `framework.ScorePlugin` under `scheduler/plugins/`; enable + weight via gatewayConfiguration.yaml. Random plugin is auto-removed if mixed with others.
- Streaming: honor `stream: true`; router parses token usage in SSE and JSON responses.

## Integration Points
- PD disaggregation: pair prefill/decode with `ModelServer.spec.workloadSelector.pdGroup` (groupKey + label selectors). Aggregated mode uses `Context.BestPods`; disaggregated sets `PrefillPods`/`DecodePods`.
- KV connectors: chosen by `ModelServer.spec.kvConnector.type` (`http|lmcache|nixl|mooncake`). Add via `connectors.KVConnector` + registration in `connectors/factory.go`.
- Rate limiting: configure per model in `ModelRoute.spec.rateLimit` with per‑unit tokens; optional global Redis at `global.redis.address`. Gateway tracks input/output tokens (including streams).
- AuthN: JWT middleware only for `/v1/*`; JWKS rotation runs in background (see `pkg/infer-gateway/filters/auth`).

## Examples
- Per‑model limit: `examples/infer-gateway/ModelRouteWithRateLimit.yaml`.
- Global Redis limit: `examples/infer-gateway/ModelRouteWithGlobalRateLimit.yaml`.
- PD disaggregation pairing: `examples/infer-gateway/ModelServer-ds1.5b-pd-disaggragation.yaml`.

## Local Run Tips
- Needs kubeconfig + `/etc/config/gatewayConfiguration.yaml`. Build and run:
  - `go build -o bin/infer-gateway cmd/infer-gateway/main.go`
  - `ENABLE_FAIRNESS_SCHEDULING=true bin/infer-gateway --port 8080` (TLS optional).

## Changing CRDs or Request Flow
- After CRD edits: `make generate`, update examples/Helm CRDs, then `make gen-docs`.
- Keep request flow: Parse → Auth → RateLimit → Schedule → Proxy (SSE/JSON) → Record usage/post‑hooks.
