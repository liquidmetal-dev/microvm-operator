# microvm-operator

A simple kubernetes operator to create batches of MicroVMs on Flintlock-running
devices.

## Getting Started

For now refer to the auto-generated [kubebuilder docs](/docs/kubebuilder.md).

## Contributing

Refer to the general [Liquid Metal contribution guides](https://weaveworks-liquidmetal.github.io/site/docs/category/guide-for-contributors/).

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.
