# KubeClaw

Kubernetes operator for deploying secure, enterprise-ready OpenClaw instances.

## Overview

KubeClaw wraps OpenClaw in Kubernetes-native infrastructure, providing:
- **Helm Chart** for easy deployment
- **Kubernetes Operator** with CRDs for managing OpenClaw instances
- **Security Guardrails** inspired by NemoClaw/NVIDIA OpenShell
- **Policy Enforcement** for enterprise deployments
- **Workspace Bootstrapping** from private Git repositories

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                      │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              KubeClaw Operator                       │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐ │   │
│  │  │   OpenClaw  │  │   Policy    │  │   Workspace  │ │   │
│  │  │   CRD       │  │   Engine    │  │   Bootstrap  │ │   │
│  │  └─────────────┘  └─────────────┘  └──────────────┘ │   │
│  └─────────────────────────────────────────────────────┘   │
│                            │                               │
│  ┌─────────────────────────▼─────────────────────────────┐   │
│  │              OpenClaw Instances (Pods)               │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌────────────┐ │   │
│  │  │Instance │ │Instance │ │Instance │ │Instance    │ │   │
│  │  │   A     │ │   B     │ │   C     │ │   D        │ │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └────────────┘ │   │
│  │  • Sandboxed containers                               │   │
│  │  • Policy-enforced network egress                     │   │
│  │  • Workspace git clone on startup                     │   │
│  │  • Resource limits & quotas                           │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Kubernetes 1.25+
- Helm 3.12+
- kubectl configured

### Install KubeClaw Operator

```bash
helm repo add kubeclaw https://egkristi.github.io/kubeclaw
helm repo update
helm install kubeclaw-operator kubeclaw/kubeclaw-operator \
  --namespace kubeclaw-system \
  --create-namespace
```

### Deploy an OpenClaw Instance

```yaml
apiVersion: kubeclaw.io/v1alpha1
kind: OpenClaw
metadata:
  name: my-assistant
  namespace: default
spec:
  # Workspace configuration
  workspace:
    repository: https://github.com/egkristi/munin-openclaw-workspace
    branch: main
    credentials:
      secretRef: workspace-git-credentials
    
  # Model configuration
  model:
    provider: anthropic
    apiKeySecretRef: anthropic-api-key
    model: claude-sonnet-4-6
    
  # Security policies
  security:
    sandbox:
      enabled: true
      landlock: true
      seccomp: true
      networkPolicy: true
    egress:
      mode: whitelist
      allowedDomains:
        - api.anthropic.com
        - github.com
        - raw.githubusercontent.com
    
  # Resource limits
  resources:
    requests:
      cpu: "2"
      memory: 4Gi
    limits:
      cpu: "4"
      memory: 8Gi
      ephemeralStorage: 20Gi
      
  # Channels
  channels:
    telegram:
      enabled: true
      tokenSecretRef: telegram-bot-token
    email:
      enabled: true
      smtpSecretRef: email-credentials
```

Apply:
```bash
kubectl apply -f my-assistant.yaml
```

## Security Features

### Policy Engine

KubeClaw implements security policies inspired by NemoClaw/NVIDIA OpenShell:

| Feature | Description |
|---------|-------------|
| **Landlock** | Filesystem sandboxing per instance |
| **Seccomp** | System call filtering |
| **NetworkPolicy** | Kubernetes native egress control |
| **Resource Quotas** | CPU/memory/ephemeral storage limits |
| **Pod Security** | Restricted security context |

### Enterprise Guardrails

- **Audit Logging**: All agent actions logged to centralized system
- **Secret Management**: Kubernetes secrets for credentials
- **RBAC Integration**: Role-based access to OpenClaw instances
- **Policy Enforcement**: Prevent unauthorized tool usage

## CRD Reference

### OpenClawSpec

| Field | Type | Description |
|-------|------|-------------|
| `workspace.repository` | string | Git URL for workspace |
| `workspace.branch` | string | Git branch (default: main) |
| `workspace.credentials` | SecretRef | Git credentials |
| `model.provider` | string | anthropic, openai, ollama |
| `model.apiKeySecretRef` | string | Secret name for API key |
| `model.model` | string | Model identifier |
| `security.sandbox.enabled` | boolean | Enable sandboxing |
| `security.sandbox.landlock` | boolean | Enable Landlock |
| `security.egress.mode` | string | whitelist or deny-all |
| `resources` | ResourceRequirements | K8s resource specs |

## Development

```bash
# Build operator
cd operator/
go build -o bin/kubeclaw-operator .

# Run locally (requires kubeconfig)
make run

# Build Helm chart
cd chart/
helm package .
```

## License

MIT - See LICENSE
