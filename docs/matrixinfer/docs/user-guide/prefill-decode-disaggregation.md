import QuickStartYaml from '../../../../examples/model/pd-disaggregation.yaml?raw';
import CodeBlock from '@theme/CodeBlock';

# Prefill-Decode Disaggregation

## Overview

Matrixinfer support vLLM PD disaggregation feature, which allows users to offload the prefill and decode stages of a
model inference to different sets of GPUs. This can help optimize resource utilization and improve performance for
large-scale inference workloads. For more details, please refer to the vLLM
documentation:  [Disaggregated Prefill](https://docs.vllm.ai/en/stable/features/disagg_prefill.html)

## Quick Start

An example configuration for prefill-decode disaggregation is as follows:

<CodeBlock language="yaml">
{QuickStartYaml}
</CodeBlock>

```bash
kubectl apply -f https://raw.githubusercontent.com/matrixinfer-ai/matrixinfer/refs/heads/main/examples/model/pd-disaggregation.yaml
```

## Architecture

<!-- Add architecture details here -->

## Configuration

<!-- Add configuration options here -->

## Benefits

<!-- Add benefits here -->