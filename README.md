# microvm-operator

A simple kubernetes operator to create batches of MicroVMs on Flintlock-running
devices.

**Note that this is VERY poc right now...**

## Getting Started

For now refer to the auto-generated [kubebuilder docs](/docs/kubebuilder.md).

### Microvm spec

The `Microvm` spec (and the `template.spec` of `MicrovmReplicaSet`,
`MicrovmDeployment` and `MicrovmTemplate`) embeds the shared
[controller-pkg](https://github.com/liquidmetal-dev/controller-pkg) microvm spec.
As well as the usual `vcpu`, `memoryMb`, `kernel`, `rootVolume`,
`additionalVolumes` and `networkInterfaces` fields it supports:

- `provider`: the microvm provider to use on the host. Unset means the host's default.
- `allowGuestAgent`: attach a vsock device so the in-guest guest-agent can talk to the host.
- `cpuConfig.featuresToEnable` / `cpuConfig.kvmCapabilitiesToDisable`: CPU feature tuning; values are provider specific.
- `additionalVolumes[].mountPoint`: guest path at which to mount the volume.
- `additionalVolumes[].virtiofsPath`: back the volume with a virtiofs share instead of a container `image`.

These fields need flintlock v0.16.0 or newer on the host. See
[config/samples](/config/samples) for examples.

## Contributing

Refer to the general [Liquid Metal contribution guides](https://weaveworks-liquidmetal.github.io/site/docs/category/guide-for-contributors/).

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.
