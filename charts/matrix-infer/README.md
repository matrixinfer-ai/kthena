# Introduction

This is the [helm](https://helm.sh/) that MatrixInfer used to deploy on Kubernetes.

Files in `crds/` are custom resource definitions, which are used to define the custom resources used by MatrixInfer.

`Chart.yaml` is a YAML file containing information about the chart.
Visit [here](https://helm.sh/docs/topics/charts/#the-chartyaml-file) for more information.

`charts/` is a directory containing the dependencies of the chart. There are two subcharts `registry` and `workload` in
it.

`values.yaml` is a YAML file containing the default configuration values for this chart.

`templates/` is a directory containing the Kubernetes manifests that will be deployed by this chart.

# Usage

prerequisite

- [helm](https://helm.sh/docs/intro/install/) installed

## package

package chart into an archive file

```shell
cd charts/
helm package matrix-infer
```

## install

> Helm installs resources in the following order:
> Namespace  
> NetworkPolicy  
> ResourceQuota  
> LimitRange  
> PodSecurityPolicy  
> PodDisruptionBudget  
> ServiceAccount  
> Secret  
> SecretList  
> ConfigMap  
> StorageClass  
> PersistentVolume  
> PersistentVolumeClaim  
> CustomResourceDefinition  
> ClusterRole  
> ClusterRoleList  
> ClusterRoleBinding  
> ClusterRoleBindingList  
> Role  
> RoleList  
> RoleBinding  
> RoleBindingList  
> Service  
> DaemonSet  
> Pod  
> ReplicationController  
> ReplicaSet  
> Deployment  
> HorizontalPodAutoscaler  
> StatefulSet  
> Job  
> CronJob  
> Ingress  
> APIService

NOTICE: The current version helm (v3.17.0) will not update or uninstall CRD. If you want to update or uninstall CRD, you
need to do it manually.

### install from local archive

```shell
helm install <your-name> <archive-file-name> --namespace <namespace> 
```

## uninstall

```shell
helm uninstall <your-name>
```

## test

```shell
cd charts/matrix-infer
helm lint
```
