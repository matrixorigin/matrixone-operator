# Matrixone Operator

[![LICENSE](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)

- It is built using the [kubebuilder](https://book.kubebuilder.io/)
- Matrixone Operator provisions and manages [Matrixone](https://github.com/matrixorigin/matrixone) on [Kubernetes](https://kubernetes.io/)

## Quick start

You can follow our [Get Started](./docs/getting_started.md) guilde to quick start a testing cluster and play with Matrixone Operator on your own machine.

## Contributing

Contributions are welcome and greatly appreciated. See [develop guide](./docs/develop_guide.md) for details about Matrixone Operator develop story ideas.

## Notice
- The Operator currently runs on TKE/EKS. GKE, AKC, and other Managed Public Cloud k8s have not been tested (but We provide the deployment method of helm). Welcome to try on other cloud platform and give a hand to make matrixone operator take a step forward. 
- multi-region is not supported yet, but you can schedule pod to different region by [nodeSelector configuration options](https://github.com/matrixorigin/matrixone-operator/blob/main/docs/api.md#nodeselector). 
- The Operator only supported 1 nodes cluster with `matrixone:0.4.0`, you can try `matrixone:kc-0.3.0` for 3 nodes cluster. We will support elastic horizontal scalability in the immediate future. 
- The Operator does not yet include monitor, istio, etc. 

## License

Matrixone Operator is under the Apache 2.0 license. See the [LICENSE](./LICENSE) file for details.
