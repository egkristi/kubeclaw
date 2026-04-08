# kubeclaw

> **OpenClaw multi-tenancy on Kubernetes — simple and secure.**

kubeclaw is a Kubernetes operator that makes it easy to run multiple OpenClaw
instances on a single cluster, with proper isolation, security, and lifecycle
management built in.

One operator. Any cluster size — from a Raspberry Pi 5 with k3s to a
datacenter with dozens of high-spec nodes.

---

## What It Does

Without kubeclaw, running multiple OpenClaw instances on Kubernetes requires
manually managing namespaces, RBAC, secrets, network policies, resource quotas,
and pod security — for every instance. Get one thing wrong and tenants bleed
into each other.

kubeclaw handles all of that declaratively:

```yaml
# This is all you need to get a secure, isolated OpenClaw instance:
apiVersion: kubeclaw.io/v1alpha1
kind: OpenClaw
metadata:
  name: munin
  namespace: tenant-erling
spec:
  model:
    provider: anthropic
    apiKeySecretRef: anthropic-key
  workspace:
    repository: https://github.com/egkristi/munin-openclaw-workspace
    credentials:
      name: git-credentials
  resources:
    limits:
      cpu: "2"
      memory: "2Gi"
```

kubeclaw takes care of the rest:

- ✅ Isolated namespace per tenant
- ✅ NetworkPolicy: default deny, explicit allow
- ✅ Pod Security: non-root, read-only rootfs, all capabilities dropped
- ✅ ResourceQuota enforced per tenant
- ✅ Secrets managed (optionally via Vault / External Secrets)
- ✅ Policy validation before anything deploys (Kyverno)
- ✅ Lifecycle management: upgrades, restarts, status tracking

---

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│  Kubernetes Cluster                                        │
│                                                            │
│  kubeclaw-operator                                         │
│  ├── Reconciles OpenClaw CRs                              │
│  ├── Reconciles OpenClawTenant CRs                        │
│  └── Enforces policy via admission webhooks               │
│                                                            │
│  Kyverno (policy engine)                                  │
│  ├── Validates all OpenClaw resources before apply        │
│  ├── Enforces security context on all pods                │
│  └── Blocks SSRF, metadata access, missing limits         │
│                                                            │
│  ┌──────────────────────┐  ┌──────────────────────┐       │
│  │  Namespace: tenant-a  │  │  Namespace: tenant-b  │       │
│  │  ┌──────────────────┐ │  │  ┌──────────────────┐ │       │
│  │  │  OpenClaw (pod)  │ │  │  │  OpenClaw (pod)  │ │       │
│  │  │  - non-root      │ │  │  │  - non-root      │ │       │
│  │  │  - readOnlyRootFS│ │  │  │  - readOnlyRootFS│ │       │
│  │  │  - caps: none    │ │  │  │  - caps: none    │ │       │
│  │  └──────────────────┘ │  │  └──────────────────┘ │       │
│  │  NetworkPolicy: ✅     │  │  NetworkPolicy: ✅     │       │
│  │  ResourceQuota: ✅     │  │  ResourceQuota: ✅     │       │
│  │  Secrets: isolated ✅  │  │  Secrets: isolated ✅  │       │
│  └──────────────────────┘  └──────────────────────┘       │
│                                                            │
│  Tenants are fully isolated from each other.               │
│  One tenant cannot see, reach, or affect another.         │
└────────────────────────────────────────────────────────────┘
```

---

## Multi-Tenancy Model

A **tenant** is an isolated unit — a team, a user, or a project.
Each tenant gets their own namespace with scoped resources and permissions.

```yaml
apiVersion: kubeclaw.io/v1alpha1
kind: OpenClawTenant
metadata:
  name: team-alpha
spec:
  displayName: "Team Alpha"

  # Resource budget for this tenant
  quota:
    maxInstances: 5
    cpu: "8"
    memory: "8Gi"
    storage: "20Gi"

  # Network: what can instances in this tenant reach?
  network:
    allowedEgress:
      - host: "api.anthropic.com"
        port: 443
      - host: "api.openai.com"
        port: 443
    denyMetadataAccess: true    # Block 169.254.169.254 etc.

  # Secrets backend for this tenant
  secretStore:
    kind: ClusterSecretStore
    name: vault-backend
```

Operator provisions automatically:
- Namespace `tenant-team-alpha`
- RBAC scoped to namespace
- NetworkPolicy: default deny + tenant allowlist
- ResourceQuota from tenant spec
- LimitRange defaults for pods

---

## Security: What kubeclaw Enforces

kubeclaw enforces security at multiple layers. These are not optional.

### Pod Security (always on)

Every OpenClaw pod, always:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: [ALL]
  seccompProfile:
    type: RuntimeDefault
```

### Kyverno Policies (bundled with Helm chart)

| Policy | What it enforces |
|--------|-----------------|
| `require-security-context` | Non-root, readOnlyRootFS, drop ALL caps |
| `require-resource-limits` | CPU + memory limits mandatory |
| `block-cloud-metadata` | Block 169.254.169.254 and equivalents |
| `validate-workspace-repository` | No localhost/metadata URLs in git config |
| `block-host-namespaces` | No hostPID, hostNetwork, hostIPC |
| `block-privileged` | No privileged containers |

### Admission Webhooks (operator)

Before any OpenClaw or OpenClawTenant CR is accepted:
- Workspace repository URL checked for SSRF patterns
- Resource limits verified
- Tenant quota checked (would this exceed the tenant's budget?)
- Cross-tenant references blocked

### Network Isolation

Per-tenant NetworkPolicy:
- Default: deny all ingress and egress
- Explicit allow: model provider APIs (per tenant config)
- Explicit allow: cluster-internal DNS
- Blocked: cloud metadata endpoints, host network

---

## Cluster Profiles

### Minimal — RPi5 / k3s / home server

```yaml
# helm install kubeclaw kubeclaw/kubeclaw -f values-minimal.yaml
istio:
  enabled: false        # Too heavy (~300MB), skip
kyverno:
  enabled: true         # Lightweight (~200MB), always useful
  background: false     # Only validate on create/update
externalSecrets:
  enabled: false        # Use K8s Secrets directly
```

Estimated RAM: ~700MB + ~512MB per OpenClaw instance  
Tested on: RPi5 4GB with k3s

### Standard — small team / VPS

```yaml
kyverno:
  enabled: true
externalSecrets:
  enabled: true
  backend: vault        # Self-hosted Vault
istio:
  enabled: false        # Optional at this scale
```

### Enterprise — datacenter / managed K8s

```yaml
kyverno:
  enabled: true
externalSecrets:
  enabled: true
  backend: aws-secrets-manager  # or azure-key-vault, vault
istio:
  enabled: true         # mTLS between all services
  mtls: STRICT
  egressGateway: true
```

---

## Optional: KlawAgent Plugin

A standard OpenClaw installation on a single system has direct access to that
system's filesystem and processes — that's how it executes commands, reads files,
and interacts with the environment.

When OpenClaw runs in a Kubernetes pod, it has no such access (by design —
that's the point of container isolation). If you need OpenClaw to work on
systems outside the cluster, **KlawAgent** is the answer.

KlawAgent is a separate project — a lightweight agent binary installed on target
systems, accessible to OpenClaw as a tool over an encrypted tunnel.

```
# Without KlawAgent: OpenClaw in K8s can only work within its own pod
OpenClaw pod → [nothing outside cluster]

# With KlawAgent (optional plugin):
OpenClaw pod → KlawAgent (E2E encrypted) → target system
                                            (policy-controlled)
```

KlawAgent is not required to use kubeclaw. It is an optional extension for
teams that need OpenClaw to act on systems outside the K8s cluster.

See: [github.com/egkristi/klawagent](https://github.com/egkristi/klawagent)

---

## Install

```bash
# Add Helm repo
helm repo add kubeclaw https://egkristi.github.io/kubeclaw
helm repo update

# Install (minimal — works on RPi5)
helm install kubeclaw kubeclaw/kubeclaw \
  --namespace kubeclaw-system \
  --create-namespace \
  -f values-minimal.yaml

# Verify
kubectl get pods -n kubeclaw-system

# Deploy your first OpenClaw instance
kubectl apply -f examples/openclaw-basic.yaml
```

---

## CRDs

| CRD | Purpose |
|-----|---------|
| `OpenClaw` | A single OpenClaw instance |
| `OpenClawTenant` | A tenant — isolation boundary, quota, network rules |

---

## Documentation

| Document | Contents |
|----------|---------|
| [docs/architecture.md](docs/architecture.md) | Full architecture and security layers |
| [docs/multi-tenancy.md](docs/multi-tenancy.md) | Tenant model, isolation guarantees |
| [docs/security.md](docs/security.md) | All security controls, Kyverno policies |
| [docs/roadmap-with-klawagent.md](docs/roadmap-with-klawagent.md) | Development roadmap |
| [LICENSING.md](LICENSING.md) | AGPLv3 + Commercial dual-license |

---

## License

AGPLv3 (open source core) + Commercial (enterprise).  
See [LICENSING.md](LICENSING.md).

---

## Related

- **[OpenClaw](https://github.com/openclaw/openclaw)** — the AI agent platform kubeclaw orchestrates
- **[KlawAgent](https://github.com/egkristi/klawagent)** 🔒 — optional plugin for remote system access
