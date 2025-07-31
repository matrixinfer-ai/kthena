# Introduction

This is the [helm](https://helm.sh/) that MatrixInfer used to deploy on Kubernetes.

Files in `crds/` are custom resource definitions, which are used to define the custom resources used by MatrixInfer.

`Chart.yaml` is a YAML file containing information about the chart.
Visit [here](https://helm.sh/docs/topics/charts/#the-chartyaml-file) for more information.

`charts/` is a directory containing the dependencies of the chart. There are three subcharts `registry`, `workload` and
`gateway` in
it.

`values.yaml` is a YAML file containing the default configuration values for this chart.

`templates/` is a directory containing the Kubernetes manifests that will be deployed by this chart.

# Usage

## Prerequisites

- [Helm](https://helm.sh/docs/intro/install/) must be installed.
- [cert-manager](https://cert-manager.io/docs/installation/) is optional and only required if you enable it by setting `global.certManager.enable` to `true` in `values.yaml`.


## Install


Syntax:
```shell
helm install <release-name> <chart> [flags]
```
Example:
```shell
helm install my-release my-chart --namespace my-namespace --create-namespace
```

### Installation Customization
You can override the default value of `values.yaml` by using the `--set` flag and `-f`.

```shell
helm install <release-name> <chart> --namespace <namespace> \
  --set registry.enabled=true \
  --set networking.enabled=false \
  -f values-dev.yaml # And you can even specify a customized `values.yaml` file for installation.
```

### Installation Order

Helm first installs resources from the `/crd` directory.  
After that, it installs resources from the `/templates` directory in the following order:
> - Namespace
> - NetworkPolicy
> - ResourceQuota
> - LimitRange
> - PodSecurityPolicy
> - PodDisruptionBudget
> - ServiceAccount
> - Secret
> - SecretList
> - ConfigMap
> - StorageClass
> - PersistentVolume
> - PersistentVolumeClaim
> - CustomResourceDefinition  (CRD)
> - ClusterRole
> - ClusterRoleList
> - ClusterRoleBinding
> - ClusterRoleBindingList
> - Role
> - RoleList
> - RoleBinding
> - RoleBindingList
> - Service
> - DaemonSet
> - Pod
> - ReplicationController
> - ReplicaSet
> - Deployment
> - HorizontalPodAutoscaler
> - StatefulSet
> - Job
> - CronJob
> - Ingress
> - APIService

**NOTICE:**  
HELM manages the installation of CRDs. However, if you need to uninstall or update a CRD, please use `kubectl apply` or `kubectl delete`.   
For more details on the reasoning behind this, see [this explanation](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations) and [these limitations](https://helm.sh/docs/topics/charts/#limitations-on-crds).

## Uninstall

```shell
helm uninstall <release-name> -n <namespace>
```

## Test

### Lint
```shell
helm lint charts/matrixinfer
```
### Validate
```shell
helm template test-release charts/matrixinfer --validate
```
### Debug
```shell
helm template test-release charts/matrixinfer --debug
```

### Dry-run
```shell
helm install test-release charts/matrixinfer --dry-run
```

## Distribution

You have two options for distributing the Helm chart:

### Option 1: Package the Chart as an Archive

Package your chart into a `.tgz` archive file using the following command:

```shell
helm package charts/matrixinfer
```

This creates a versioned archive (e.g., `matrixinfer-0.1.0.tgz`) that can be easily shared or uploaded to a Helm repository.

### Option 2: Generate a Single Manifest File

You can generate a single YAML manifest file (`install.yaml`) from your Helm chart. This file can be applied directly with `kubectl`.

**Without CRDs:**
```shell
helm template my-release charts/matrixinfer > install.yaml
```

**With CRDs included:**
```shell
helm template my-release charts/matrixinfer --include-crds > install.yaml
```

> **Note:** Use the `--include-crds` flag if you want to include Custom Resource Definitions (CRDs) in the generated manifest.
