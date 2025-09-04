# minfer - MatrixInfer CLI

`minfer` is a command-line interface tool for managing MatrixInfer AI inference workloads in Kubernetes clusters.

## Architecture Diagrams

### Use Case Diagram

```plantuml
actor "User" as user

package "minfer CLI" {
  
  package "Verb Layer" {
    usecase "Get" as GetVerb
    usecase "Describe" as DescribeVerb
    usecase "Create" as CreateVerb
  }
  
  package "Resource Layer" {
    usecase "Templates" as Templates
    usecase "Template" as Template
    
    usecase "Manifest" as Manifest
    
    rectangle "Kubernetes Resources" as KubernetesResources {
       usecase "Models" as ModelResources
       usecase "Model" as ModelResource
       usecase "ModelInfers" as ModelInferResources
       usecase "ModelInfer" as ModelInferResource
       usecase "Policy" as PolicyResource
       usecase "Policies" as PolicyResources
    }
  }
  
  package "Flag Layer" {
    usecase "-o yaml" as OutputFlag
    usecase "--dry-run" as DryRunFlag
    usecase "--namespace" as NamespaceFlag
    usecase "--all-namespaces" as AllNamespacesFlag
  }
}

' User interactions with verb layer (kubectl-style)
user --> GetVerb : minfer get
user --> DescribeVerb : minfer describe
user --> CreateVerb : minfer create

' Verb layer connects to resource layer
CreateVerb --> Manifest : manifest
GetVerb --> Templates : templates
GetVerb --> Template : template [NAME]
GetVerb --> ModelResources : models
GetVerb --> ModelInferResources : modelinfers
GetVerb --> PolicyResources : autoscaling-policies

DescribeVerb --> Template : template [NAME]
DescribeVerb --> ModelResources : model [NAME]
DescribeVerb --> ModelInferResources : modelinfer [NAME]

' Resources can use flags
Templates --> OutputFlag
Templates --> DryRunFlag
ModelResources --> NamespaceFlag
ModelResources --> AllNamespacesFlag
ModelInferResources --> AllNamespacesFlag
ModelInferResources --> NamespaceFlag
PolicyResources --> AllNamespacesFlag
PolicyResources --> NamespaceFlag

note left of Templates
  Browse and view templates with
  kubectl-style verb-noun syntax
end note

note top of CreateVerb
  Create MatrixInfer resources from
  templates with custom values
end note

note right of KubernetesResources
  Manage Model, ModelInfer, AutoscalingPolicy
  resources using kubectl-style commands
end note
```

## Overview

The `minfer` CLI follows kubectl-style verb-noun grammar and provides an easy way to:
- Get and view templates and MatrixInfer resources in your cluster
- Describe detailed information about specific templates and resources
- Create MatrixInfer resources from predefined manifest templates
- Manage inference workloads, models, and autoscaling policies with kubectl-like commands

### Build from Source

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

The `minfer` CLI follows kubectl-style verb-noun grammar for consistency and ease of use.

### Template Operations

List all available templates:
```bash
minfer get templates
```

Get a specific template content:
```bash
minfer get template deepseek-r1-distill-llama-8b
minfer get template deepseek-r1-distill-llama-8b -o yaml
```

Describe a template with detailed information:
```bash
minfer describe template deepseek-r1-distill-llama-8b
```

### Resource Operations

List models:
```bash
minfer get models
minfer get models --all-namespaces
minfer get models -n production
```

List model inference workloads:
```bash
minfer get modelinfers
minfer get modelinfers --all-namespaces
```

List autoscaling policies:
```bash
minfer get autoscaling-policies
minfer get autoscaling-policies -n production
```

### Creating Resources

Create resources from templates:
```bash
minfer create manifest --template deepseek-r1-distill-llama-8b --name my-model
minfer create manifest --template deepseek-r1-distill-llama-8b --values-file values.yaml
minfer create manifest --template deepseek-r1-distill-llama-8b --name my-model --dry-run
```

For more detailed usage information, run:
```bash
minfer --help
minfer get --help
minfer describe --help
minfer create --help
```

## Configuration

The CLI uses your local kubectl configuration. Ensure you have:
- Valid kubeconfig file (usually `~/.kube/config`)
- Access to a Kubernetes cluster with MatrixInfer CRDs installed
- Appropriate RBAC permissions for the target namespaces

## Contributing

To add new manifest templates:

1. Create a new `.yaml` file in the `templates/` directory
2. Use Go template syntax with variables: `{{.variable_name}}`
3. Add a description comment at the top: `# Description: Your template description`
4. Test with `minfer describe template your-template`

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

## About Cobra

Minfer CLI is built with [Cobra](https://github.com/spf13/cobra).
The building blocks of cobra are depicted as below.

```plantuml
!theme plain
skinparam linetype ortho
skinparam nodesep 10
skinparam ranksep 20

' Main tree structure
rectangle "**Cobra CLI Application**\n//Entry Point//" as App #lightblue

rectangle "**Root Command**\n//cobra.Command//\n- Entry point\n- Execute()\n- Global configuration" as Root #lightgreen

rectangle "**Sub Command 1**\n//cobra.Command//\n- Specific functionality\n- Inherits from parent" as Sub1 #lightgreen

rectangle "**Sub Command 2**\n//cobra.Command//\n- Different functionality\n- Can have own subcommands" as Sub2 #lightgreen

rectangle "**Sub-Sub Command**\n//cobra.Command//\n- Nested functionality\n- Deep command hierarchy" as SubSub #lightgreen

' Flags branch
rectangle "**Flags**\n//Command Options//" as FlagsRoot #lightyellow
rectangle "**Persistent Flags**\n- Available to all subcommands\n- Inherited down the tree\n- Global configuration" as PFlags #lightyellow
rectangle "**Local Flags**\n- Command-specific only\n- Not inherited\n- Local configuration" as LFlags #lightyellow

' Arguments branch
rectangle "**Arguments**\n//Positional Parameters//" as ArgsRoot #lightcyan
rectangle "**Validation Rules**\n- NoArgs\n- MinimumNArgs(n)\n- ExactArgs(n)\n- Custom validation" as ArgValidation #lightcyan
rectangle "**Argument Values**\n- []string args\n- Parsed by cobra\n- Passed to Run function" as ArgValues #lightcyan

' Hooks/Lifecycle branch
rectangle "**Lifecycle Hooks**\n//Execution Flow//" as Hooks #lightpink
rectangle "**Pre-Execution**\n- PersistentPreRun\n- PreRun\n- Setup and validation" as PreHooks #lightpink
rectangle "**Execution**\n- Run function\n- Main command logic\n- Core functionality" as RunHook #lightpink
rectangle "**Post-Execution**\n- PostRun\n- PersistentPostRun\n- Cleanup and finalization" as PostHooks #lightpink

' External Integration branch
rectangle "**External Integration**\n//Third-party Libraries//" as External #lavender
rectangle "**Viper**\n- Configuration management\n- File/env/flag binding\n- Hot reloading" as Viper #lavender
rectangle "**Pflag**\n- POSIX-compliant flags\n- Flag parsing engine\n- Type conversion" as Pflag #lavender

' Features branch
rectangle "**Features**\n//Built-in Capabilities//" as Features #wheat
rectangle "**Auto Help**\n- Generated help text\n- Usage information\n- Command discovery" as Help #wheat
rectangle "**Shell Completion**\n- Bash completion\n- Zsh completion\n- PowerShell completion" as Completion #wheat
rectangle "**Error Handling**\n- Command validation\n- Flag validation\n- Custom error messages" as ErrorHandling #wheat

' Main tree connections
App ||--|| Root : "starts with"
Root ||--o{ Sub1 : "contains"
Root ||--o{ Sub2 : "contains"
Sub1 ||--o{ SubSub : "can contain"

' Flags connections
Root ||--|| FlagsRoot : "has"
FlagsRoot ||--|| PFlags : "includes"
FlagsRoot ||--|| LFlags : "includes"

' Arguments connections
Root ||--|| ArgsRoot : "validates"
ArgsRoot ||--|| ArgValidation : "applies"
ArgsRoot ||--|| ArgValues : "processes"

' Hooks connections
Root ||--|| Hooks : "executes"
Hooks ||--|| PreHooks : "runs first"
Hooks ||--|| RunHook : "runs main"
Hooks ||--|| PostHooks : "runs last"

' External connections
Root ||..|| External : "integrates"
External ||--|| Viper : "uses"
External ||--|| Pflag : "built on"

' Features connections
Root ||--|| Features : "provides"
Features ||--|| Help : "generates"
Features ||--|| Completion : "supports"
Features ||--|| ErrorHandling : "handles"

' Inheritance flows (dotted lines for inheritance)
Sub1 -.-> Root : "inherits flags & hooks"
Sub2 -.-> Root : "inherits flags & hooks"
SubSub -.-> Sub1 : "inherits flags & hooks"

note top of App
**Execution Flow:**
main() → rootCmd.Execute() → 
flag parsing → argument validation → 
hook execution → command logic
end note

note bottom of PFlags
**Flag Inheritance:**
Persistent flags flow down
the command tree to all
child commands
end note

note right of Hooks
**Hook Execution Order:**
1. PersistentPreRun (parent)
2. PreRun (current command)
3. Run (main logic)
4. PostRun (current command)
5. PersistentPostRun (parent)
end note

```
