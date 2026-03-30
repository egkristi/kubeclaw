# KubeClaw Operator

Kubernetes operator for deploying secure OpenClaw instances.

## Quick Start

```bash
# Build the operator
make build

# Run locally (requires kubeconfig)
make run

# Build container image
make docker-build IMG=ghcr.io/egkristi/kubeclaw-operator:v0.1.0

# Deploy to cluster
make deploy IMG=ghcr.io/egkristi/kubeclaw-operator:v0.1.0
```

## Development

```bash
# Run tests
make test

# Generate manifests (CRDs, RBAC)
make manifests

# Generate deepcopy code
make generate
```

## Helm Installation

```bash
make helm-install
```
