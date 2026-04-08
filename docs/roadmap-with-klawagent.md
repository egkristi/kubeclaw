# kubeclaw — Oppdatert Utviklingsplan med KlawAgent

**Dato:** April 8, 2026  
**Status:** Design + delvis implementert  
**Kontekst:** Planoppdatering etter KlawAgent-design

---

## Hva kubeclaw Er — Revidert Definisjon

kubeclaw er ikke bare en Kubernetes-operator for OpenClaw.

Med KlawAgent-integrasjon blir kubeclaw en **komplett enterprise AI-agent-plattform**:

```
kubeclaw = OpenClaw orchestration on K8s
         + KlawAgent: reach to ANY system outside the cluster
         + Unified security policy across cluster and targets
         + Multi-tenant isolation
         + Zero-ingress networking
```

```
┌──────────────────────────────────────────────────────────────┐
│  Kubernetes Cluster (zero ingress, no public IP needed)      │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  kubeclaw-operator                                      │  │
│  │  - Orkestrerer OpenClaw-instanser (tenants)            │  │
│  │  - Distribuerer policy til KlawAgents                  │  │
│  │  - Håndterer External Secrets (Vault/KV)               │  │
│  │  - Istio mTLS mellom alle pods                         │  │
│  └────────────────────┬───────────────────────────────────┘  │
│                        │                                      │
│  ┌─────────────────────▼──────────────────────────────────┐  │
│  │  Tenant: acme                                           │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐  │  │
│  │  │  OpenClaw-1  │  │  OpenClaw-2  │  │ KlawAgent   │  │  │
│  │  │  (AI agent)  │  │  (AI agent)  │  │  Gateway    │  │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬──────┘  │  │
│  └──────────┼────────────────┼─────────────────┼──────────┘  │
└─────────────┼────────────────┼─────────────────┼─────────────┘
              │                │                 │ Noise XX + relay
              └────────────────┴─────────────────┘
                                                 │
                    ┌────────────────────────────┘
                    ▼
        ┌───────────────────────┐
        │  Target systems       │
        │  (behind NAT/FW)      │
        │  ┌───────┐ ┌───────┐  │
        │  │Agent  │ │Agent  │  │
        │  │Linux  │ │Win    │  │
        │  └───────┘ └───────┘  │
        └───────────────────────┘
```

---

## Grunnpremisser: Kubernetes som Sikkerhetsplattform

Kubernetes gir isolasjon gratis. kubeclaw arver og forsterker dette.

### Hva K8s allerede leverer

| Lag | K8s standard | kubeclaw arver |
|-----|-------------|----------------|
| **Container-isolasjon** | Namespace, cgroups, seccomp | ✅ + PSA restricted |
| **Nettverksisolasjon** | NetworkPolicy per namespace | ✅ + Istio mTLS |
| **Secret management** | etcd-kryptert | ✅ + External Secrets (Vault) |
| **RBAC** | K8s native | ✅ + kubeclaw-spesifikk |
| **ResourceQuota** | Per namespace | ✅ konfigurert per tenant |
| **Pod Security** | PSA restricted | ✅ påkrevd for alle pods |

### Hva kubeclaw legger til

| Lag | kubeclaw tillegg |
|-----|-----------------|
| **Service Mesh** | Istio mTLS mellom alle tjenester |
| **External Systems** | KlawAgent — E2E-kryptert tilgang utenfor cluster |
| **Policy Distribution** | Kyverno for K8s-native policy + KlawAgent RPCPolicy |
| **Secret Rotation** | External Secrets Operator med Vault |
| **Audit** | App-level audit på RPC-kall + K8s audit |
| **Multi-tenancy** | OpenClawTenant CRD med namespace-isolering |

---

## NemoClaw vs. kubeclaw — Oppdatert Sammenligning

NemoClaw er single-host sandboxing. kubeclaw er enterprise orchestration.
De er ikke i samme kategori — og med KlawAgent er gapet enda større.

### Arkitektonisk premiss

```
NemoClaw:
  1 host → Landlock sandbox → 1 OpenClaw
  Alle ressurser på én maskin. Ingen distribusjon.

kubeclaw:
  K8s cluster → Namespace-isolert → N OpenClaw-instanser
  + KlawAgent → M target systems utenfor cluster
  Distribuert, skalerbart, enterprise-grade.
```

### Detaljert sammenligning

| Sikkerhetslag | NemoClaw | kubeclaw (med KlawAgent) | Vinner |
|---------------|----------|--------------------------|--------|
| **Container-isolasjon** | seccomp/Landlock på vert | K8s namespace + PSA restricted | 🏆 kubeclaw (dypere) |
| **Nettverksisolasjon** | Linux netns + iptables | Istio mTLS + AuthorizationPolicy | 🏆 kubeclaw |
| **Service auth** | Ingen | SPIFFE/SVID identiteter | 🏆 kubeclaw |
| **Egress-kontroll** | iptables per sandbox | Istio Egress Gateway + Kyverno | 🏆 kubeclaw |
| **External system access** | Landlock på vert | KlawAgent: Noise XX E2E + policy | 🏆 kubeclaw |
| **Cross-system kryptering** | Ingen (lokal) | Noise XX, relay-opaque | 🏆 kubeclaw |
| **NAT-traversal** | ❌ | ✅ via KlawAgent cascade | 🏆 kubeclaw |
| **Air-gap support** | ❌ | ✅ Reticulum/serial | 🏆 kubeclaw |
| **Secret management** | Lokale filer | Vault + External Secrets Operator | 🏆 kubeclaw |
| **Secret rotation** | Manuell | Automatisk (ESO refreshInterval) | 🏆 kubeclaw |
| **Policy enforcement** | Hardkodet | Kyverno (deklarativ YAML) | 🏆 kubeclaw |
| **Blast radius** | N/A (single host) | Politikkbegrenset (maxConcurrentAgents) | 🏆 kubeclaw |
| **Audit logging** | Basic | App-level + K8s audit + RPC-kall | 🏆 kubeclaw |
| **SSRF-beskyttelse** | Landlock | Kyverno policy + Istio + KlawAgent deny | 🏆 kubeclaw |
| **Multi-tenant** | ❌ | ✅ OpenClawTenant CRD | 🏆 kubeclaw |
| **Skalerbarhet** | 1 sandbox/host | Ubegrenset | 🏆 kubeclaw |
| **RPi5/k3s support** | ✅ | ✅ (lettvekt-konfig) | 🟡 Tie |
| **Routed inference** | ✅ | ✅ via KlawAgent proxy | 🟡 Tie |

**Score: kubeclaw 16 — NemoClaw 1 — Tie 2**

### Hva NemoClaw gjør bedre (ærlig vurdering)

- **Enklere å sette opp** — single binary på én host. Zero K8s-kompleksitet.
- **Lavere overhead** — ingen K8s, ingen Istio, ingen operator.
- **Kernel-level sandboxing** — Landlock er dypere enn container-isolasjon for filsystem.

kubeclaw er riktig valg for: enterprise, multi-tenant, multi-system, team.  
NemoClaw er riktig valg for: én person, én host, maksimal enkelhet.

---

## Revidert CRD-Modell med KlawAgent

### Nye og oppdaterte CRD-er

```
kubeclaw.io/v1alpha1
├── OpenClaw              (eksisterende — OpenClaw-instans)
├── OpenClawTenant        (eksisterende — tenant-isolasjon)
├── KlawAgentConfig       (NY — agent-registrering og policy)
├── ConnectionPolicy      (NY — fra KlawAgent)
├── RPCPolicy             (NY — fra KlawAgent)
├── SecurityPolicy        (NY — cluster-wide immutable rules)
└── DesiredStatePolicy    (NY — convergence-regler)
```

### KlawAgentConfig CRD

```yaml
apiVersion: kubeclaw.io/v1alpha1
kind: KlawAgentConfig
metadata:
  name: prod-server-1
  namespace: tenant-acme
spec:
  agentID: prod-server-1
  displayName: "Production Server 1"
  labels:
    environment: production
    role: web-server
    security-zone: high

  # Hvilke policies gjelder
  connectivityPolicyRef: high-security
  rpcPolicyRef: production
  metricsPolicyRef: full

  # Bootstrap-token konfig
  bootstrap:
    tokenTTL: 1h

status:
  connected: true
  lastSeen: "2026-04-08T02:00:00Z"
  activeTransport: wireguard-relay
  agentVersion: "0.1.0"
  hostname: prod-web-1
  os: linux/amd64
```

### OpenClaw CRD (utvidet)

```yaml
apiVersion: kubeclaw.io/v1alpha1
kind: OpenClaw
metadata:
  name: main
  namespace: tenant-acme
spec:
  model:
    provider: anthropic
    apiKeySecretRef: model-credentials  # Via External Secrets → Vault

  workspace:
    repository: https://github.com/acme/workspace
    credentials:
      name: git-credentials            # Via External Secrets → Vault

  # KlawAgent-integrasjon (NY)
  klawagent:
    enabled: true
    allowedAgents:
      - "*"                            # Alle agenter i denne tenant
    # Eller spesifikke:
    # - prod-server-1
    # - build-win-1

  # External Secrets backend
  secretStore:
    kind: ClusterSecretStore
    name: vault-backend
    refreshInterval: 1h

  # Resource limits (Kyverno enforcer)
  resources:
    limits:
      cpu: "2"
      memory: "2Gi"
    requests:
      cpu: "500m"
      memory: "512Mi"
```

---

## Revidert Implementasjonsplan

### Fase 1 — K8s-kjerne (allerede påbegynt, ~20h gjenstår)

**Status:** CRDs definert, reconciler delvis implementert.

**Gjenstår:**
- [ ] Validation webhooks (4h) — avvis dårlige CRs før apply
- [ ] OpenClawTenant controller — full namespace-provisjonering (4h)
- [ ] Kyverno policy-bundle medfølger Helm-chart (4h)
- [ ] External Secrets Operator-integrasjon (4h)
- [ ] Helm chart ferdigstilt og testet (4h)

**Output:** Fungerende kubeclaw uten KlawAgent. Kan deploye OpenClaw på K8s med multi-tenant.

### Fase 2 — KlawAgent-integrasjon (ny, ~30h)

**Avhenger av:** KlawAgent MVP (WebSocket + Noise XX + RPCPolicy) — ~44h.

**kubeclaw-siden:**
- [ ] KlawAgentConfig CRD + controller (6h)
  - Provisjonerer KlawAgent-namespace
  - Genererer bootstrap-token
  - Distribuerer ConnectivityPolicy og RPCPolicy til agenten
  - Tracker agent-status (connected, lastSeen, activeTransport)
- [ ] KlawAgent Gateway deployment (4h)
  - Relay-broker som K8s Deployment
  - Meet-token generator (control plane)
  - Agent-registry
- [ ] OpenClaw → KlawAgent tool registration (6h)
  - OpenClaw-instanser i kubeclaw får automatisk tilgang til KlawAgents i sin tenant
  - Tool-definisjon injiseres i OpenClaw-config
- [ ] Policy distribution (6h)
  - Operator pusher ConnectivityPolicy og RPCPolicy til tilkoblede agenter
  - Hot-reload uten agent-restart
- [ ] Metrics aggregering (4h)
  - KlawAgent-metrikk samles inn og eksponeres til Prometheus/InfluxDB
- [ ] CLI: `kubectl kubeclaw agent` subcommands (4h)
  - `token generate`, `list`, `status`, `revoke`

**Output:** OpenClaw-instanser i kubeclaw kan RPC til target-systemer. AI-agenter kan arbeide på systemer utenfor clusteret, policy-kontrollert.

### Fase 3 — Istio + Service Mesh (ny, ~16h)

- [ ] Istio-installasjon som valgfri kubeclaw-avhengighet (2h)
- [ ] PeerAuthentication: STRICT mTLS per tenant-namespace (2h)
- [ ] AuthorizationPolicy: default-deny + intra-tenant allow (2h)
- [ ] Egress Gateway: whitelist modell-providers, blokkere metadata (4h)
- [ ] Rate limiting per tenant (2h)
- [ ] Observability: Kiali + Jaeger-integrasjon (4h)

### Fase 4 — Desired State og Playbooks (via KlawAgent, ~20h)

Når KlawAgent v0.2+ har desired state og playbook-støtte:

- [ ] PlaybookJob CRD — kjør KlawAgent-playbook fra K8s (8h)
- [ ] DesiredStateCheck CRD — kontinuerlig drift-deteksjon (8h)
- [ ] Resultat-aggregering og alerting (4h)

### Fase 5 — Enterprise (fremtidig)

- [ ] Headscale-integrasjon (self-hosted WireGuard control plane)
- [ ] Compliance-rapportering (SOC2-ready audit log)
- [ ] Multi-cluster federation
- [ ] Helm chart på ArtifactHub

---

## Lettvekt-konfig: RPi5 + k3s

kubeclaw er designet for å kjøre på RPi5 med k3s uten full Istio:

```yaml
# values-rpi5.yaml — minimal footprint
global:
  platform: k3s

istio:
  enabled: false          # For tung for RPi5 (~300MB)

kyverno:
  enabled: true           # Lettvekt (~200MB)
  background: false       # Kun validate on create/update

klawagent:
  enabled: true
  relay:
    enabled: true         # Self-hosted relay
    replicas: 1

resources:
  operator:
    limits:
      cpu: "500m"
      memory: "256Mi"
  relay:
    limits:
      cpu: "100m"
      memory: "64Mi"
```

**Estimert RAM på RPi5 (4GB):**
```
k3s:              ~512MB
kubeclaw-operator: ~128MB
Kyverno:          ~200MB
KlawAgent relay:   ~64MB
OpenClaw (1x):    ~512MB
─────────────────────────
Total:           ~1.4GB   (2.6GB ledig for OS + ekstra instanser)
```

---

## Prioritert Rekkefølge (hva å bygge når)

```
Nå (P0 — parallelt med KlawAgent MVP):
  kubeclaw Fase 1 — validation webhooks + Kyverno + ESO + Helm (~20h)

Etter KlawAgent MVP (44h):
  kubeclaw Fase 2 — KlawAgent-integrasjon (~30h)

  → Demo: "OpenClaw på k3s/RPi5 med KlawAgent på laptop"
  → Alle kommandoer kryptert, policy-kontrollert, auditert

Deretter:
  kubeclaw Fase 3 — Istio (kun for cloud/enterprise) (~16h)
  kubeclaw Fase 4 — Desired State + Playbooks (~20h)
```

**Total gjenstående for full kubeclaw + KlawAgent MVP:**
```
kubeclaw Fase 1+2:  ~50h
KlawAgent MVP:      ~44h
─────────────────────────
Total:             ~94h  (~2.5 måneder deltid)
```

---

## Det Unike Verdiforslaget

Etter Fase 1+2:

> **kubeclaw er det eneste Kubernetes-operatøren som gir AI-agenter
> policy-kontrollert, E2E-kryptert tilgang til systemer utenfor clusteret —
> gjennom hvilken som helst nettverkstopologi, uten inbound porter.**

Ingen andre verktøy kombinerer:
- K8s-native AI-agent-orkestrering
- Zero-ingress networking
- E2E-kryptert remote execution
- Deklarativ policy på alle lag
- Støtte fra RPi5-k3s til enterprise EKS/GKE/AKS

---

*Plan: Munin for Erling Kristiansen — April 8, 2026*
