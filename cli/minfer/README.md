# minfer - MatrixInfer CLI

`minfer` is a command-line interface tool for managing MatrixInfer AI inference workloads in Kubernetes clusters.

## Architecture Diagrams

### Use Case Diagram

```plantuml
actor "User" as user

package "minfer CLI" {
  
  package "Action Layer" {
    usecase "List" as ListAction
    usecase "Create" as CreateAction
  }
  
  package "Target Layer" {
    usecase "Templates" as TemplatesTarget
    rectangle "Matrixinfer Resources" as Resources {
       usecase "Models" as ModelsTarget
       usecase "ModelInfers" as ModelInfersTarget
       usecase "Policies" as PoliciesTarget
    }
  }
  
  package "Flag Layer" {
    usecase "--describe" as DescribeFlag
    usecase "--dry-run" as DryRunFlag
    usecase "--namespace" as NamespaceFlag
    usecase "--all-namespaces" as AllNamespacesFlag
  }
}

' User interactions with action layer
user --> ListAction : minfer list
user --> CreateAction : minfer create

' Action layer connects to target layer
ListAction --> TemplatesTarget : templates
ListAction --> ModelsTarget : models
ListAction --> ModelInfersTarget : modelinfers
ListAction --> PoliciesTarget : autoscaling-policies

CreateAction --> TemplatesTarget : manifest

' Targets can use flags
TemplatesTarget --> DescribeFlag
TemplatesTarget --> DryRunFlag
ModelsTarget --> NamespaceFlag
ModelsTarget --> AllNamespacesFlag
ModelInfersTarget --> AllNamespacesFlag
ModelInfersTarget --> NamespaceFlag
PoliciesTarget --> AllNamespacesFlag
PoliciesTarget --> NamespaceFlag

note left of TemplatesTarget
  Browse available manifest templates
  with descriptions and variables
end note

note top of CreateAction
  Create MatrixInfer resources from
  templates with custom values
end note

note right of Resources
  List Model, ModelInfer, AutoscalingPolicy
  and AutoscalingPolicyBinding resources
end note
```

## Overview

The `minfer` CLI provides an easy way to:
- Create MatrixInfer resources from predefined manifest templates
- List and view existing MatrixInfer resources in your cluster
- Manage inference workloads, models, and autoscaling policies
- Apply configurations with template rendering and user confirmation

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

Please see `minfer --help`.

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
