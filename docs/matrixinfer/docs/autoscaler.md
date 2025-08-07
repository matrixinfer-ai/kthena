# MatrixInfer Autoscaler

## 概述
MatrixInfer Autoscaler 是一个为 Kubernetes 上的模型推理工作负载设计的智能自动扩缩容组件。它能够根据实时指标动态调整模型实例数量，在确保性能需求的同时优化资源利用率。该组件特别适合大语言模型(LLM)服务，支持多种扩缩容策略和灵活的配置选项。

## 支持的自动扩缩容机制
MatrixInfer Autoscaler 目前支持以下自动扩缩容机制：

### 基于指标的自动扩缩容
这种机制依赖于运行时指标（如 CPU/GPU 利用率、并发请求数等）来做出扩缩容决策，能够立即响应系统负载的变化。

#### 标准自动扩缩容
基于 Kubernetes HPA（水平 Pod 自动扩缩器）原理，根据 CPU、内存或自定义指标扩展 Pod 副本数。适合具有稳定使用模式的通用工作负载。

#### 智能自动扩缩容
MatrixInfer 增强版自动扩缩器，专为 LLM 工作负载设计。具有以下特点：
- 双窗口机制（稳定 + 紧急），在流量突然激增时实现快速扩容
- 波动容差，减少不必要的扩缩容震荡
- 直接从模型服务获取指标，无需依赖外部监控系统

## 示例场景

### 标准自动扩缩容示例
适用于 CPU 密集型的模型推理服务，根据 CPU 利用率自动调整实例数量：
```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: AutoscalingPolicyBinding
metadata:
  name: cpu-based-scaling
spec:
  scalingConfiguration:
    minReplicas: 1
    maxReplicas: 5
    target:
      targetRef:
        apiVersion: workload.matrixinfer.ai/v1alpha1
        kind: ModelInfer
        name: cpu-model
    metricTargets:
      cpu: 70
```

### 智能自动扩缩容示例
适用于流量波动较大的 LLM 服务，配置紧急模式以应对突发流量：
```yaml
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: llm-smart-scaling
spec:
  tolerancePercent: 10
  behavior:
    scaleUp:
      panicPolicy:
        panicThresholdPercent: 150
        panicModeHold: 5m
    scaleDown:
      stabilizationWindow: 3m
---
apiVersion: registry.matrixinfer.ai/v1alpha1
kind: AutoscalingPolicyBinding
metadata:
  name: smart-scaling-binding
spec:
  policyRef:
    name: llm-smart-scaling
  scalingConfiguration:
    minReplicas: 2
    maxReplicas: 10
    target:
      targetRef:
        apiVersion: workload.matrixinfer.ai/v1alpha1
        kind: ModelInfer
        name: llm-model
    metricTargets:
      gpu_utilization: 80
      concurrent_requests: 50
```

## 扩缩容方法
MatrixInfer Autoscaler 支持以下扩缩容方法：

### 反应式扩缩容
观察当前系统指标并立即做出反应，向上或向下扩展资源。这包括所有基于指标的方法：
- 监控实时指标（CPU、内存、GPU 利用率等）
- 当指标超过或低于阈值时触发扩缩容
- 考虑容差范围和稳定窗口，避免频繁调整

### 工作流程
自动扩缩容的工作流程如下：
1. **指标收集**：从运行中的 Pod 收集指标数据
2. **推荐计算**：根据指标和策略计算推荐实例数
3. **决策修正**：考虑紧急模式和历史记录修正推荐
4. **实例更新**：更新 ModelInfer 的副本数量

## 核心算法

### 推荐实例数算法
推荐实例数算法考虑以下因素：
- 当前实例数量
- 最小/最大实例限制
- 指标目标值和容差范围
- 就绪/未就绪实例数量
- 外部指标

```go
func (alg *RecommendedInstancesAlgorithm) GetRecommendedInstances() (recommendedInstances int32, skip bool) {
    // 检查边界条件
    if alg.CurrentInstancesCount < alg.MinInstances {
        return alg.MinInstances, false
    }
    if alg.CurrentInstancesCount > alg.MaxInstances {
        return alg.MaxInstances, false
    }
    
    // 计算推荐实例数...
}
```

### 紧急模式
当需要快速扩容时（如流量激增），Autoscaler 会进入紧急模式，忽略常规限制以快速响应需求。紧急模式持续时间可配置。

## 指标
MatrixInfer Autoscaler 与服务层紧密集成，直接消费模型引擎暴露的指标。支持的指标类型包括：

### 系统指标
- `cpu_utilization`: CPU 利用率百分比
- `memory_usage`: 内存使用量
- `gpu_utilization`: GPU 利用率百分比
- `gpu_memory_usage`: GPU 内存使用量

### 业务指标
- `concurrent_requests`: 并发请求数
- `request_count`: 请求总数
- `token_in_count`: 输入 token 数
- `token_out_count`: 输出 token 数
- `inference_latency_ms`: 推理延迟（毫秒）

## 配置说明

### 主要配置参数

#### AutoscalingPolicy 配置
- `tolerancePercent`: 指标容差百分比，避免因微小波动触发扩缩容
- `behavior.scaleUp.panicPolicy.panicThresholdPercent`: 触发紧急模式的阈值
- `behavior.scaleUp.panicPolicy.panicModeHold`: 紧急模式持续时间
- `behavior.scaleDown.stabilizationWindow`: 缩容稳定窗口

#### AutoscalingPolicyBinding 配置
- `minReplicas`: 最小实例数
- `maxReplicas`: 最大实例数
- `target.targetRef`: 目标 ModelInfer 引用
- `metricTargets`: 指标目标值映射

## 扩展或选择自动扩缩器
MatrixInfer 允许您在部署规范中声明性地选择所需的自动扩缩器类型。这使得您可以轻松尝试不同的策略，或随着基础设施的发展集成新的自动扩缩插件。

## 注意事项
1. 确保模型服务暴露了符合 Prometheus 格式的指标端点
2. 合理设置最小/最大实例数，避免资源浪费或性能不足
3. 根据实际工作负载调整容差范围和稳定窗口
4. 监控 autoscaler 自身的指标，确保其正常工作
5. 对于 GPU 密集型工作负载，建议监控 GPU 利用率和内存使用情况