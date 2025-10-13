# Prepare for Development

This document helps you get started developing code for kthena.
If you follow this guide and find some problem, please take
a few minutes to update this file.

Kthena components only have few external dependencies you
need to set up before being able to build and run the code.

- [Prepare for Development](#prepare-for-development)
  - [Setting up Go](#setting-up-go)
  - [Setting up Docker](#setting-up-docker)
  - [Setting up Kubernetes](#setting-up-kubernetes)
  - [Setting up a personal access token](#setting-up-a-personal-access-token)

## Setting up Go

All kthena components are written in the [Go](https://golang.org) programming language.
To build, you'll need a Go development environment. If you haven't set up a Go development
environment, please follow [these instructions](https://golang.org/doc/install)
to install the Go tools.

Kthena currently builds with Go 1.24.0

## Setting up Docker

Kthena has a Docker build system for creating and publishing Docker images.
To leverage that you will need:

- **Docker platform:** To download and install Docker follow [these instructions](https://docs.docker.com/install/).

- **Docker Hub:**  GitHub provides container image services. You can access them directly using your GitHub account. Alternatively, you can push built Kthena images to your private repository.

## Setting up Kubernetes

We require Kubernetes version 1.28 or higher with CRD support.

If you aren't sure which Kubernetes platform is right for you, see [Picking the Right Solution](https://kubernetes.io/docs/setup/).

- [Installing Kubernetes with Minikube](https://kubernetes.io/docs/setup/learning-environment/minikube/)

- [Installing Kubernetes with kops](https://kubernetes.io/docs/setup/production-environment/tools/kops/)

- [Installing Kubernetes with kind](https://kind.sigs.k8s.io/)

## Setting up a personal access token

This is only necessary for core contributors in order to push changes to the main repos.
You can make pull requests without two-factor authentication
but the additional security is recommended for everyone.

To be part of the Volcano organization, we require two-factor authentication, and
you must setup a personal access token to enable push via HTTPS. Please follow
[these instructions](https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/)
for how to create a token.
Alternatively you can [add your SSH keys](https://help.github.com/articles/adding-a-new-ssh-key-to-your-github-account/).
