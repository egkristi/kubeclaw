# KubeClaw Development Guide

## Project Structure

```
kubeclaw/
├── README.md
├── LICENSE
├── Makefile
├── operator/                    # Kubernetes operator (Go)
│   ├── api/
│   │   └── v1alpha1/          # CRD Go types
│   ├── internal/
│   │   ├── controller/        # Reconciliation logic
│   │   └── webhooks/          # Validation webhooks
│   ├── cmd/
│   │   └── manager/           # Operator entrypoint
│   └── pkg/
│       ├── policy/            # Policy engine
│       ├── sandbox/           # Sandbox management
│       └── workspace/         # Git clone utilities
├── chart/                     # Helm chart
│   └── kubeclaw-operator/
│       ├── Chart.yaml
│       ├── values.yaml
│       ├── crds/
│       └── templates/
├── examples/                  # Example manifests
├── docs/                      # Documentation
└── hack/                      # Development scripts
```

## Building

### Prerequisites

- Go 1.21+
- Docker
- kubectl with access to a cluster
- Helm 3.12+

### Local Development

```bash
# Run operator locally (requires kubeconfig)
cd operator/
go run cmd/manager/main.go

# Run tests
make test

# Build container image
make docker-build IMG=ghcr.io/egkristi/kubeclaw-operator:v0.1.0

# Push image
make docker-push IMG=ghcr.io/egkristi/kubeclaw-operator:v0.1.0
```

### Deploy to Cluster

```bash
# Install CRDs
make install

# Deploy operator
make deploy IMG=ghcr.io/egkristi/kubeclaw-operator:v0.1.0

# Or use Helm
helm install kubeclaw-operator ./chart/kubeclaw-operator \
  --namespace kubeclaw-system \
  --create-namespace
```

## Security Architecture

### Sandboxing Layers

1. **Container-level**: Restricted security context, read-only root filesystem
2. **Landlock**: Filesystem access policies per OpenClaw instance
3. **Seccomp**: System call filtering
4. **NetworkPolicy**: Kubernetes-native egress control

### Policy Enforcement

The operator enforces policies at multiple levels:
- **Admission**: Webhook validates OpenClaw CRs before admission
- **Runtime**: Security contexts applied to pods
- **Network**: NetworkPolicies restrict egress
- **Resource**: Resource quotas prevent DoS

## Testing

```bash
# Unit tests
make test

# Integration tests (requires kind cluster)
make test-integration

# E2E tests
make test-e2e
```

## Contributing

See CONTRIBUTING.md for guidelines.

## License

Apache 2.0
