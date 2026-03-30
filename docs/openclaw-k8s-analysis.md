# KubeClaw Design Analysis: OpenClaw Kubernetes Patterns

## Key Insights from OpenClaw Official K8s Docs

### Current OpenClaw K8s Architecture

```
Namespace: openclaw
├── Deployment/openclaw          # Single pod
│   ├── Init container           # Setup/config
│   └── Gateway container          # OpenClaw runtime
├── Service/openclaw             # ClusterIP:18789
├── PersistentVolumeClaim        # 10Gi for state
├── ConfigMap/openclaw-config    # openclaw.json + AGENTS.md
└── Secret/openclaw-secrets      # Gateway token + API keys
```

### Deployment Method: Kustomize (Not Helm)

OpenClaw uses **Kustomize** for simplicity:
- No Helm chart overhead
- Base + overlays pattern
- Easy customization via patches

**KubeClaw Decision:** Provide BOTH:
- Helm chart for enterprise users
- Kustomize base for simple deployments

### Security Model

| Feature | OpenClaw Implementation | KubeClaw Enhancement |
|---------|------------------------|---------------------|
| **Root FS** | `readOnlyRootFilesystem: true` | ✅ Same + Landlock |
| **Capabilities** | `drop: ALL` | ✅ Same |
| **User** | UID 1000 (non-root) | ✅ Same |
| **Network** | Loopback bind only | ✅ NetworkPolicy egress |
| **Secrets** | K8s Secrets | ✅ Sealed Secrets option |
| **Sandbox** | Container only | ✅ + seccomp + Landlock |

### Access Pattern

**OpenClaw Default:**
```
kubectl port-forward svc/openclaw 18789:18789
→ http://localhost:18789 (loopback only)
```

**For Remote Access:**
- HTTPS/Tailscale Serve
- Proper gateway bind config
- Control UI origin settings

**KubeClaw Enhancement:**
- Ingress support with TLS
- Service mesh integration (Istio/Linkerd)
- Built-in oauth2-proxy for auth

### Storage Model

| Aspect | OpenClaw | KubeClaw |
|--------|----------|----------|
| **PVC** | 10Gi default | Configurable |
| **Contents** | Agent state + config | + Git workspace |
| **Backup** | Manual | Velero integration |
| **HA** | Single pod | StatefulSet option |

### Configuration Management

**OpenClaw Pattern:**
```yaml
ConfigMap:
  - openclaw.json        # Gateway config
  - AGENTS.md            # Bootstrap content
```

**KubeClaw Enhancement:**
```yaml
ConfigMap:
  - openclaw.json        # Base gateway config
  - AGENTS.md            # Bootstrap (optional)
  
Secret:
  - workspace-git-credentials    # Git SSH key
  - provider-api-keys            # Anthropic/OpenAI/etc
  - channel-credentials          # Telegram/email
```

### Workspace Bootstrap

**OpenClaw:** Manual — edit ConfigMap, redeploy

**KubeClaw:** Automated via CRD
```yaml
spec:
  workspace:
    repository: https://github.com/egkristi/munin-openclaw-workspace
    branch: main
    credentials:
      secretRef: workspace-git-credentials
```

Init container clones repo before gateway starts.

### Model Provider Handling

**OpenClaw:**
- Environment variables: `ANTHROPIC_API_KEY`, etc.
- Script generates K8s Secret
- Multiple providers supported

**KubeClaw:**
```yaml
spec:
  model:
    provider: anthropic
    apiKeySecretRef: anthropic-api-key
    model: claude-sonnet-4-6
```

### What KubeClaw Adds

| Feature | OpenClaw | KubeClaw |
|---------|----------|----------|
| **Multi-instance** | ❌ Single | ✅ CRD-based |
| **Operator** | ❌ None | ✅ Full operator |
| **Policy Engine** | ❌ Basic | ✅ Enterprise policies |
| **Sandbox** | ❌ Container | ✅ Landlock+seccomp+net |
| **Observability** | ❌ None | ✅ Prometheus/OTel |
| **GitOps** | ❌ Manual | ✅ ArgoCD ready |
| **Multi-tenant** | ❌ Single | ✅ Namespace isolation |

### Design Decisions for KubeClaw

#### 1. Keep OpenClaw Gateway Container

Don't rewrite — wrap and enhance:
- Use official `ghcr.io/openclaw/openclaw` image
- Add init containers for setup
- Add sidecars for observability

#### 2. CRD-Driven Configuration

Instead of editing ConfigMaps:
```yaml
apiVersion: kubeclaw.io/v1alpha1
kind: OpenClaw
metadata:
  name: my-assistant
spec:
  # All config here
```

#### 3. Secrets Handling

**Option A:** K8s Secrets (current)
**Option B:** External Secrets Operator
**Option C:** Vault integration

#### 4. Network Architecture

```
┌─────────────────────────────────────────┐
│           Ingress (TLS)                 │
│     (oauth2-proxy for auth)            │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│     OpenClaw Service (ClusterIP)        │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│     OpenClaw Pod                        │
│  ┌──────────┐ ┌──────────┐ ┌─────────┐ │
│  │  Init    │ │  Gateway │ │ Sidecar │ │
│  │ (git)    │ │ (core)   │ │ (otel)  │ │
│  └──────────┘ └──────────┘ └─────────┘ │
└─────────────────────────────────────────┘
```

#### 5. Storage Strategy

**Default:** PVC per instance
**Alternative:**
- EmptyDir (ephemeral, for CI/testing)
- ConfigMap/Secret mounts (static configs)
- External storage (S3 for backups)

### Implementation Checklist

- [ ] Init container: Git clone workspace
- [ ] Init container: Landlock policy setup
- [ ] Security context: seccomp profile
- [ ] NetworkPolicy: egress whitelist
- [ ] Service: ClusterIP default
- [ ] Ingress: Optional with TLS
- [ ] Monitoring: Prometheus metrics
- [ ] Logging: Structured JSON
- [ ] Backup: Velero annotations

### Migration Path

Existing OpenClaw K8s users → KubeClaw:

```bash
# 1. Export current config
kubectl get configmap openclaw-config -o yaml > backup.yaml

# 2. Create OpenClaw CR from config
kubectl apply -f - <<EOF
apiVersion: kubeclaw.io/v1alpha1
kind: OpenClaw
metadata:
  name: migrated-assistant
spec:
  # Copy config from backup.yaml
EOF

# 3. Migrate PVC data (if needed)
# 4. Decommission old deployment
```

## Summary

KubeClaw builds ON TOP of OpenClaw's Kubernetes foundation:
- **Preserves:** Security hardening, PVC model, port-forward workflow
- **Enhances:** Multi-instance CRDs, policy engine, observability
- **Adds:** Enterprise features without complexity for simple use cases

The goal: Make OpenClaw Kubernetes-native, not Kubernetes-complex.
