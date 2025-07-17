# Introduction

---

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

---

prerequisite

- [helm](https://helm.sh/docs/intro/install/) installed

## Package

package chart into an archive file

```shell
cd charts/
helm package matrix-infer
```

## Install

```shell
helm install <release-name> <chart> [flags]
```

### Installation Customization
By default, all subcharts will be installed. If you want to specify which of them to install, you can customize by using the `--set` flag.

```shell
# this will only install workload subchart, and disable registry and gateway subcharts
hell install <your-name> <archive-file-name> --namespace <namespace> \
  --set registry.enabled=false,gateway.enabled=false
# or
hell install <your-name> <archive-file-name> --namespace <namespace> \
  --set registry.enabled=false \
  --set gateway.enabled=false
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
HELM manages the installation of CRDs. However, if you need to uninstall or update a CRD, please use `kubectl apply` or `kubectl delete`. For more details on the reasoning behind this, see [this explanation](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations).

## Uninstall

```shell
helm uninstall <your-name>
```

## Test

```shell
cd charts/matrix-infer
helm lint
```
