# minfer - MatrixInfer CLI

`minfer` is a command-line interface tool for managing MatrixInfer AI inference workloads in Kubernetes clusters.

## Architecture Diagrams

### Use Case Diagram

```plantuml
@startuml
!define RECTANGLE class

actor "DevOps Engineer" as user
actor "ML Engineer" as mluser
actor "Kubernetes Cluster" as k8s

rectangle "minfer CLI" {
  usecase "List Templates" as UC1
  usecase "Describe Template" as UC2
  usecase "Create Manifest" as UC3
  usecase "List Resources" as UC4
  usecase "Dry Run Preview" as UC5
}

user --> UC1 : minfer list templates
user --> UC2 : minfer list templates --describe
user --> UC3 : minfer create manifest
user --> UC4 : minfer list modelinfers/models
user --> UC5 : minfer create manifest --dry-run

mluser --> UC1
mluser --> UC2
mluser --> UC3
mluser --> UC5

UC3 --> k8s : Apply resources
UC4 --> k8s : Query resources

note right of UC1
  Browse available manifest templates
  with descriptions and variables
end note

note right of UC3
  Create MatrixInfer resources from
  templates with custom values
end note

note right of UC4
  List ModelInfer, Model, AutoscalingPolicy
  and AutoscalingPolicyBinding resources
end note

@enduml
```

### Sequence Diagram - Create Manifest Flow

```plantuml
@startuml
participant "User" as U
participant "minfer CLI" as CLI
participant "Template Engine" as TE
participant "Kubernetes API" as K8S

U -> CLI: minfer create manifest --template basic-inference --set name=my-model
CLI -> CLI: Load template values from flags/file
CLI -> TE: Get embedded template content
TE -> CLI: Return template YAML
CLI -> TE: Render template with values
TE -> CLI: Return rendered YAML
CLI -> U: Display generated YAML
U -> CLI: Confirm application (y/N)
CLI -> K8S: Apply ModelInfer resource
K8S -> CLI: Resource created
CLI -> K8S: Apply Model resource  
K8S -> CLI: Resource created
CLI -> K8S: Apply AutoscalingPolicy resource
K8S -> CLI: Resource created
CLI -> K8S: Apply AutoscalingPolicyBinding resource
K8S -> CLI: Resource created
CLI -> U: All resources applied successfully!

alt Dry Run Mode
  U -> CLI: minfer create manifest --dry-run
  CLI -> U: Display YAML (no application)
end

alt Template not found
  CLI -> U: Error: template 'name' not found
end

@enduml
```

### Component Diagram

```plantuml
@startuml
!define COMPONENT component

package "minfer CLI" {
  COMPONENT [Root Command] as root
  COMPONENT [Create Command] as create
  COMPONENT [List Command] as list
  COMPONENT [Template Engine] as templates
  
  package "Embedded Templates" {
    COMPONENT [basic-inference.yaml] as basic
    COMPONENT [simple-model.yaml] as simple
  }
  
  package "Resource Managers" {
    COMPONENT [ModelInfer Manager] as mi
    COMPONENT [Model Manager] as model
    COMPONENT [AutoscalingPolicy Manager] as asp
    COMPONENT [AutoscalingPolicyBinding Manager] as aspb
  }
}

package "External Dependencies" {
  COMPONENT [Kubernetes Client] as k8sclient
  COMPONENT [MatrixInfer Client] as miclient
  COMPONENT [YAML Parser] as yaml
  COMPONENT [Template Parser] as tmpl
}

root --> create
root --> list
create --> templates
list --> templates
list --> mi
list --> model
list --> asp
list --> aspb

templates --> basic
templates --> simple
templates --> tmpl

create --> yaml
create --> miclient
list --> miclient

miclient --> k8sclient

note top of templates
  Manages embedded template files
  and provides template rendering
  capabilities
end note

note bottom of miclient
  Provides typed clients for
  MatrixInfer custom resources
end note

@enduml
```

## Overview

The `minfer` CLI provides an easy way to:
- Create MatrixInfer resources from predefined manifest templates
- List and view existing MatrixInfer resources in your cluster
- Manage inference workloads, models, and autoscaling policies
- Apply configurations with template rendering and user confirmation

## Installation

### Prerequisites

- Go 1.24+ installed
- Kubernetes cluster with MatrixInfer CRDs installed
- kubectl configured to access your cluster
- Valid kubeconfig file

### Build from Source

```bash
# From the project root directory
make minfer
```

This will create a `minfer` binary in the `./bin` directory.

Alternatively, you can build manually:

```bash
# From the project root directory
go build -o bin/minfer cli/minfer/main.go
```

### Add to PATH (Optional)

```bash
# Add the bin directory to your PATH
export PATH=$PATH:$(pwd)/bin
```

## Usage

### Basic Commands

```bash
# Show help
minfer --help

# List available manifest templates
minfer list templates

# Show detailed information about a specific template
minfer list templates --describe basic-inference

# List MatrixInfer resources
minfer list modelinfers
minfer list models
minfer list autoscaling-policies
```

### Creating Resources from Manifests

The `minfer create manifest` command allows you to create MatrixInfer resources from predefined templates.

#### Basic Usage

```bash
# Create a basic inference workload
minfer create manifest \
  --template basic-inference \
  --set name=my-model \
  --set image=my-registry/model:v1.0 \
  --set model_name=my-model

# Use dry-run to preview without applying
minfer create manifest \
  --template basic-inference \
  --set name=my-model \
  --set image=nginx:latest \
  --set model_name=test-model \
  --dry-run

# Create from values file
minfer create manifest \
  --template basic-inference \
  --values-file values.yaml

# Specify target namespace
minfer create manifest \
  --template basic-inference \
  --namespace production \
  --set name=prod-model \
  --set image=my-registry/model:v2.0 \
  --set model_name=production-model
```

#### Using Values Files

Create a `values.yaml` file:

```yaml
name: my-inference-workload
image: my-registry/pytorch-model:v1.2.3
model_name: sentiment-analysis
model_version: v1.2.3
namespace: ml-workloads
replicas: 3
memory_request: 2Gi
cpu_request: 1000m
memory_limit: 4Gi
cpu_limit: 2000m
min_replicas: 1
max_replicas: 10
target_cpu: 80
framework: pytorch
author: ml-team
description: Sentiment analysis model for customer feedback
```

Then use it:

```bash
minfer create manifest --template basic-inference --values-file values.yaml
```

### Listing Resources

```bash
# List model inference workloads in current namespace
minfer list modelinfers

# List across all namespaces
minfer list modelinfers --all-namespaces

# List in specific namespace
minfer list models --namespace production

# List all resource types
minfer list models
minfer list autoscaling-policies
minfer list autoscaling-policy-bindings
```

## Available Manifest Templates

### basic-inference

Creates a complete inference setup with:
- ModelInfer workload
- Model registration
- AutoscalingPolicy
- AutoscalingPolicyBinding

**Required Variables:**
- `name`: Name for the inference workload
- `image`: Container image for the model server  
- `model_name`: Name of the model

**Optional Variables:**
- `namespace`: Target namespace (default: "default")
- `replicas`: Initial replica count (default: 1)
- `model_version`: Model version (default: "v1.0")
- `memory_request`, `cpu_request`: Resource requests
- `memory_limit`, `cpu_limit`: Resource limits
- `min_replicas`, `max_replicas`: Autoscaling bounds
- `target_cpu`: CPU utilization target for autoscaling
- `framework`: Model framework (default: "pytorch")

### simple-model  

Creates only a Model resource for model registration.

**Required Variables:**
- `model_name`: Name of the model
- `namespace`: Target namespace

**Optional Variables:**
- `model_version`: Model version (default: "v1.0")
- `framework`: Model framework (default: "pytorch")
- `description`: Model description
- `author`: Model author
- `tags`: Model tags

## Command Reference

### Global Flags

- `--help, -h`: Show help information

### `minfer create manifest`

Create resources from a manifest template.

**Flags:**
- `--template, -t`: Template name (required)
- `--values-file, -f`: YAML file with template values
- `--set`: Set individual template values (format: key=value)
- `--namespace, -n`: Target namespace (default: "default")
- `--dry-run`: Show rendered template without applying

**Examples:**
```bash
minfer create manifest --template basic-inference --set name=test,image=nginx,model_name=test
minfer create manifest --template simple-model --values-file model-values.yaml
minfer create manifest --template basic-inference --set name=test --dry-run
```

### `minfer list`

List MatrixInfer resources.

**Subcommands:**
- `modelinfers` (aliases: `mi`, `modelinfer`): List ModelInfer resources
- `models` (alias: `model`): List Model resources  
- `autoscaling-policies` (aliases: `asp`, `autoscaling-policy`): List AutoscalingPolicy resources
- `autoscaling-policy-bindings` (aliases: `aspb`, `autoscaling-policy-binding`): List AutoscalingPolicyBinding resources

**Flags:**
- `--namespace, -n`: Target namespace
- `--all-namespaces, -A`: List across all namespaces

**Examples:**
```bash
minfer list modelinfers
minfer list models --namespace production
minfer list autoscaling-policies --all-namespaces
```

### `minfer list templates`

List and describe available manifest templates.

**Flags:**
- `--describe`: Show detailed information about a specific manifest

**Examples:**
```bash
minfer list templates
minfer list templates --describe basic-inference
```

## Workflow Example

Here's a typical workflow for creating a new inference workload:

1. **List available manifests:**
   ```bash
   minfer list templates
   ```

2. **Examine a manifest template:**
   ```bash
   minfer list templates --describe basic-inference
   ```

3. **Create and preview resources:**
   ```bash
   minfer create manifest \
     --template basic-inference \
     --set name=sentiment-model \
     --set image=my-registry/sentiment:v1.0 \
     --set model_name=sentiment-analysis \
     --dry-run
   ```

4. **Apply to cluster:**
   ```bash
   minfer create manifest \
     --template basic-inference \
     --set name=sentiment-model \
     --set image=my-registry/sentiment:v1.0 \
     --set model_name=sentiment-analysis
   ```

5. **Verify deployment:**
   ```bash
   minfer list modelinfers
   minfer list models
   ```

## Configuration

The CLI uses your local kubectl configuration. Ensure you have:
- Valid kubeconfig file (usually `~/.kube/config`)
- Access to a Kubernetes cluster with MatrixInfer CRDs installed
- Appropriate RBAC permissions for the target namespaces

## Troubleshooting

### Common Issues

**Template not found:**
```bash
Error: template 'my-template' not found at templates/my-template.yaml
```
- Check available templates with `minfer list templates`
- Ensure you're in the correct directory with the `templates/` folder

**Kubeconfig issues:**
```bash
Error: failed to load kubeconfig
```
- Verify kubectl is configured: `kubectl cluster-info`
- Check kubeconfig file permissions and location

**Resource creation failures:**
```bash
Error: failed to apply ModelInfer my-model: resources not found
```
- Ensure MatrixInfer CRDs are installed in your cluster
- Verify you have permissions to create resources in the target namespace

### Debug Mode

Use `--dry-run` to preview generated YAML without applying:

```bash
minfer create manifest --template basic-inference --set name=debug --dry-run
```

## Contributing

To add new manifest templates:

1. Create a new `.yaml` file in the `templates/` directory
2. Use Go template syntax with variables: `{{.variable_name}}`
3. Add a description comment at the top: `# Description: Your template description`
4. Test with `minfer list templates --describe your-template`

Example template structure:
```yaml
# Description: Your template description
# Variables: var1, var2, var3
---
apiVersion: workload.matrixinfer.ai/v1alpha1
kind: ModelInfer
metadata:
  name: {{.name}}
  namespace: {{.namespace}}
spec:
  # ... template content
```