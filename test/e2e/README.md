# E2E Tests for MatrixInfer

This directory contains end-to-end (E2E) tests for the MatrixInfer project using Kind (Kubernetes in Docker).

## Overview

The E2E tests will use helm to install matrixinfer into the Kind cluster and verify the core functionality.

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) must be installed
- Go 1.24
- Docker (required by Kind)
- Helm (required to install helm charts)

## Running the Tests

### Using Make (Recommended)

```bash
# Run E2E tests (automatically sets up Kind Cluster and run test)
make test-e2e

# Clean up E2E test environment (if needed)
make test-e2e-cleanup
```

## Test Environment

The tests create a Kind cluster with the following characteristics:

- **Cluster Name**: `matrixinfer-e2e` (can be overridden with `CLUSTER_NAME` env var)
- **Kubernetes Version**: v1.31.0
- **Test Namespace**: `dev`
