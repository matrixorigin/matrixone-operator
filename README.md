# Matrixone Operator

[![LICENSE](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)

- It is built using the [kubebuilder](https://book.kubebuilder.io/)
- Matrixone Operator provisions and manages [Matrixone](https://github.com/matrixorigin/matrixone) on [Kubernetes](https://kubernetes.io/)

## Quick start

You can follow our [Get Started](./docs/getting_started.md) guide to quick start a testing cluster and play with Matrixone Operator on your own machine.

## Contributing

Contributions are welcome and greatly appreciated. See [Contribution Guide](./CONTRIBUTING.md) for details about Matrixone Operator develop story ideas.

## Notice

- The Operator currently runs on TKE/EKS. GKE, AKC, and other Managed Public Cloud k8s have not been tested.
- multi-region is not supported yet.
- The default cluster size is set to 1 node temporarily, the multi-nodes cluster has not been fully tested.
- The Operator does not yet include monitor, istio, etc.

## License

Matrixone Operator is under the Apache 2.0 license. See the [LICENSE](./LICENSE) file for details.
