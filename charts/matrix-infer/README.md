# Introduction

This is the [helm](https://helm.sh/) that MatrixInfer used to deploy on Kubernetes.

Files in `crds/` are custom resource definitions, which are used to define the custom resources used by MatrixInfer.

`Chart.yaml` is a YAML file containing information about the chart.
Visit [here](https://helm.sh/docs/topics/charts/#the-chartyaml-file) for more information.

`charts/` is a directory containing the dependencies of the chart.

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