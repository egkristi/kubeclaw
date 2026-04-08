# kubeclaw — Utviklingsplan og Sikkerhetsarkitektur

**Dato:** April 8, 2026  
**Status:** Design + delvis implementert

---

## Hva kubeclaw Er

**kubeclaw er en Kubernetes-operator for OpenClaw — på enterprise-vis.**

Det betyr:
- OpenClaw kjører trygt og sikkert på Kubernetes
- Riktig isolasjon, policy, secret management, og multi-tenancy — fra dag én
- Skalerer fra én RPi5 med k3s til titalls noder i et datasenter
- Ingen kompromiss på sikkerhet, uavhengig av clusterstørrelse

**KlawAgent er ikke en del av kubeclaw.**

KlawAgent er et separat verktøy — et valgfritt plugin — som en OpenClaw-instans
kan bruke for å nå systemer utenfor clusteret. Det tilsvarer den direkte
systemtilgangen en standard OpenClaw-installasjon på ett system har, men løst
på enterprise-vis gjennom et agentlag.

```
Standard OpenClaw (single system):
  OpenClaw → exec, read, write → lokalt filsystem og prosesser

kubeclaw (K8s):
  OpenClaw pod → exec, read, write → ... ingenting utenfor clusteret (by design)
  OpenClaw pod + KlawAgent → exec, read, write → target-systemer (valgfritt plugin)
```

---

## Kubernetes som Sikkerhetsplattform

kubeclaw bygger på det Kubernetes allerede gir. Det er et bevisst valg — ikke
dupliser det K8s løser godt. Legg til det K8s ikke gir.

### Hva K8s gir gratis

| Lag | K8s standard |
|-----|-------------|
| Container-isolasjon | Namespace, cgroups, seccomp |
| Nettverksisolasjon | NetworkPolicy (med CNI) |
| Identitet og tilgangskontroll | RBAC |
| Secret management | etcd-kryptert |
| Resource-begrensning | ResourceQuota, LimitRange |
| Pod Security | Pod Security Admission (restricted) |

### Hva kubeclaw legger til

| Lag | kubeclaw tillegg | Lettvekt-alternativ |
|-----|-----------------|---------------------|
| Policy enforcement | Kyverno | Kyverno (begge) |
| Secret rotation | External Secrets Operator + Vault | ESO + lokal Vault dev |
| mTLS mellom tjenester | Istio | **Valgfritt** — kan skippes |
| Validering av CRDs | Admission webhooks | Alltid på |
| App-level audit | Structured logging | Alltid på |
| Multi-tenancy | OpenClawTenant CRD | Alltid tilgjengelig |

**Nøkkelen:** Istio er kraftig men tung (~300MB). Kyverno er lettvekt (~200MB)
og gir det meste av policy-håndhevelse. kubeclaw skal fungere utmerket uten Istio.

---

## Arkitektur

### Minimal (RPi5 / k3s / hjemmeserver)

```
┌──────────────────────────────────────────────┐
│  k3s cluster (RPi5 eller liten VPS)          │
│                                              │
│  kubeclaw-operator (~128MB)                  │
│  Kyverno (~200MB)                            │
│                                              │
│  ┌────────────────────────────────────────┐  │
│  │  Tenant: default                        │  │
│  │  ┌──────────────────────────────────┐  │  │
│  │  │  OpenClaw (pod)                   │  │  │
│  │  │  - Non-root, readOnlyRootFS       │  │  │
│  │  │  - Capabilities: ALL dropped      │  │  │
│  │  │  - NetworkPolicy: default deny    │  │  │
│  │  │  - ResourceQuota: enforced        │  │  │
│  │  └──────────────────────────────────┘  │  │
│  └────────────────────────────────────────┘  │
│                                              │
│  Total RAM: ~700MB + OpenClaw ~512MB = ~1.2GB│
└──────────────────────────────────────────────┘
```

### Full Enterprise (cloud / datacenter)

```
┌──────────────────────────────────────────────────────────────┐
│  Kubernetes Cluster (EKS / GKE / AKS / on-prem)             │
│                                                              │
│  kubeclaw-operator                                           │
│  Kyverno (policy)                                            │
│  External Secrets Operator → Vault / AWS SM / Azure KV      │
│  Istio (mTLS, egress gateway, observability) [valgfritt]     │
│                                                              │
│  ┌─────────────────────────┐  ┌─────────────────────────┐   │
│  │  Tenant: team-alpha      │  │  Tenant: team-beta       │   │
│  │  Namespace: ta           │  │  Namespace: tb           │   │
│  │  ┌───────┐ ┌───────┐    │  │  ┌───────┐ ┌───────┐    │   │
│  │  │  OC   │ │  OC   │    │  │  │  OC   │ │  OC   │    │   │
│  │  │  -1   │ │  -2   │    │  │  │  -1   │ │  -2   │    │   │
│  │  └───────┘ └───────┘    │  │  └───────┘ └───────┘    │   │
│  │  NetworkPolicy: isolated │  │  NetworkPolicy: isolated │   │
│  │  ResourceQuota: enforced │  │  ResourceQuota: enforced │   │
│  │  Secrets: via ESO        │  │  Secrets: via ESO        │   │
│  └─────────────────────────┘  └─────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

### Med KlawAgent (valgfritt plugin)

KlawAgent installeres som et separat plugin — enten i clusteret som Gateway,
eller som standalone binary på target-systemer. OpenClaw-instansen konfigureres
til å bruke KlawAgent som verktøy, akkurat som den ville brukt exec-verktøyet
lokalt på et single-system.

```
OpenClaw-pod (i kubeclaw)
  ↓ plugin kall
KlawAgent-plugin (klientbibliotek i OpenClaw)
  ↓ Noise XX kryptert
KlawAgent-relay (K8s Deployment, eller ekstern VPS)
  ↓ Noise XX kryptert
KlawAgent-binary (target-system)
  ↓ RPCPolicy-sjekk
Lokal exec / filsystem
```

---

## NemoClaw vs. kubeclaw

### Riktig premiss for sammenligningen

```
NemoClaw:   Single-host sandboxing
            1 OpenClaw ↔ 1 maskin
            Isolasjon: Landlock + seccomp på vertsOS

kubeclaw:   Kubernetes-native orchestration
            N OpenClaw ↔ K8s cluster (1 node til hundrevis)
            Isolasjon: K8s namespace + container + policy
```

De er ikke i samme kategori. NemoClaw er riktig valg for én person på én maskin.
kubeclaw er riktig valg for team, enterprise, og multi-system.

### Sikkerhetsmessig sammenligning

| Lag | NemoClaw | kubeclaw (uten Istio) | kubeclaw (med Istio) |
|-----|----------|----------------------|---------------------|
| **Prosess-isolasjon** | Landlock + seccomp | Container + PSA restricted | Container + PSA restricted |
| **Filsystem-isolasjon** | Landlock (granulær) | readOnlyRootFS + emptyDir | readOnlyRootFS + emptyDir |
| **Nettverksisolasjon** | Linux netns + iptables | NetworkPolicy | NetworkPolicy + mTLS |
| **Service-autentisering** | Ingen | RBAC | SPIFFE/SVID (mTLS) |
| **Egress-kontroll** | iptables | NetworkPolicy egress | Istio Egress Gateway |
| **Secret management** | Lokale filer | K8s Secrets | K8s Secrets + ESO + Vault |
| **Secret rotation** | Manuell | Valgfritt (ESO) | Automatisk (ESO) |
| **Policy enforcement** | Hardkodet | Kyverno (deklarativ) | Kyverno + Istio |
| **SSRF-beskyttelse** | Landlock | Kyverno NetworkPolicy | Kyverno + Istio deny |
| **Audit logging** | Basic | K8s audit + app-level | K8s audit + app-level + Istio |
| **Multi-tenant** | ❌ | ✅ Namespace-isolert | ✅ + mTLS-isolert |
| **Skalerbarhet** | 1 sandbox | Ubegrenset | Ubegrenset |
| **Kompleksitet** | Lav | Medium | Høy |
| **RPi5-kompatibel** | ✅ | ✅ | ⚠️ (krevende) |

### Hva NemoClaw gjør bedre

- **Landlock er dypere enn readOnlyRootFS** for filsystem-isolasjon.
  Landlock gir granulær path-kontroll på kernel-nivå.
  Container-isolasjon er bredere men ikke like dyp for FS-operasjoner.

- **Null K8s-overhead.** Ingen operator, ingen webhooks, ingen CRDs.
  For én person på én maskin er dette den riktige avveiningen.

- **Enklere å forstå og auditere.**
  Færre bevegelige deler = mindre angrepsflate i infrastrukturen rundt.

### Konklusjon

kubeclaw er ikke NemoClaw "gjort bedre". Det er en annen løsning for en annen
kontekst. kubeclaw utnytter det K8s allerede gir — og legger til det K8s mangler
for enterprise OpenClaw-deployments. NemoClaw gjør noe kubeclaw ikke gjør:
kernel-level Landlock sandboxing av filsystemtilgang.

Et mulig fremtidig tillegg til kubeclaw: Landlock-basert seccomp-profil for
OpenClaw-pods, som kombinerer K8s-isolasjon med NemoClaw-inspirert FS-sandboxing.

---

## Implementasjonsplan

### Fase 1 — Kjerne (P0, ~20h gjenstår)

Gjenstår av allerede påbegynt arbeid:

- [ ] Validation webhooks (4h)
  - Avvis CRs med SSRF-URLs i workspace.repository
  - Avvis CRs uten resource limits
  - Avvis CRs som bryter tenant-isolasjon
- [ ] Kyverno policy-bundle i Helm-chart (4h)
  - require-non-root, require-readonly-rootfs, drop-all-capabilities
  - block-cloud-metadata, validate-workspace-repository
  - require-resource-limits
- [ ] External Secrets Operator-integrasjon (4h)
  - ClusterSecretStore for Vault / AWS SM / Azure KV
  - ExternalSecret-templates for model-credentials og git-credentials
- [ ] Helm chart ferdigstilt (4h)
  - values-minimal.yaml (RPi5/k3s)
  - values-enterprise.yaml (cloud)
  - CRD-installasjon inkludert
- [ ] Integration tests med envtest (4h)

**Output:** Fungerende, testbar kubeclaw. Kan deploye OpenClaw på K8s med
multi-tenant, policy, og secret management.

### Fase 2 — Enterprise (P1, ~18h)

- [ ] Istio-integrasjon (valgfritt, helm flag) (6h)
  - PeerAuthentication STRICT per tenant-namespace
  - AuthorizationPolicy default-deny + intra-tenant allow
  - Egress Gateway med provider-whitelist
- [ ] Secret rotation via ESO (4h)
  - refreshInterval-basert automatisk rotation
  - Status-felt i OpenClaw CR for rotation-state
- [ ] Prometheus metrics fra operatoren (4h)
  - Antall instanser per tenant, reconcilasjonstid, feil
- [ ] Helm chart på ArtifactHub (4h)

### Fase 3 — KlawAgent-plugin (P2, ~30h)

**Avhenger av KlawAgent MVP (~44h separat).**

Integrasjon av KlawAgent som valgfritt plugin i kubeclaw:

- [ ] KlawAgentConfig CRD (6h)
  - Registrering av target-agenter per tenant
  - Bootstrap-token generering
  - Agent-status tracking
- [ ] KlawAgent Gateway (4h)
  - Relay-broker som K8s Deployment
  - Meet-token API
- [ ] Policy-distribusjon (6h)
  - Operator pusher ConnectivityPolicy + RPCPolicy til agenter
  - Hot-reload uten restart
- [ ] OpenClaw tool-registrering (6h)
  - KlawAgent-verktøy automatisk tilgjengelig i OpenClaw-config
  - Tenant-scoped: OpenClaw ser kun sine egne agenter
- [ ] `kubectl kubeclaw agent` CLI (4h)
- [ ] Metrics fra agenter (4h)

### Fase 4 — Landlock (P3, ~8h)

Kernel-level filsystem-sandboxing inspirert av NemoClaw:

- [ ] Landlock seccomp-profil for OpenClaw-pods (4h)
  - Begrenset til /workspace, /tmp, /run/openclaw
  - Init-container som setter opp Landlock-regler
- [ ] Integrasjon med SecurityContext (2h)
- [ ] Test på Linux 5.13+ (Landlock minimum) (2h)

---

## Profilvalg

### RPi5 / k3s / hobbyist

```yaml
# helm install kubeclaw kubeclaw/kubeclaw -f values-minimal.yaml
istio:
  enabled: false
kyverno:
  enabled: true
  background: false
externalSecrets:
  enabled: false    # Bruk K8s Secrets direkte
klawagent:
  enabled: false
```

RAM: ~700MB + OpenClaw-instanser

### Enterprise / cloud

```yaml
# helm install kubeclaw kubeclaw/kubeclaw -f values-enterprise.yaml
istio:
  enabled: true
kyverno:
  enabled: true
externalSecrets:
  enabled: true
  backend: vault    # vault | aws-secrets-manager | azure-key-vault
klawagent:
  enabled: true     # Valgfritt plugin
```

---

## Totalt Estimat

| Fase | Timer | Prioritet |
|------|-------|-----------|
| Fase 1 — Kjerne | ~20h | P0 |
| Fase 2 — Enterprise | ~18h | P1 |
| Fase 3 — KlawAgent plugin | ~30h | P2 |
| Fase 4 — Landlock | ~8h | P3 |
| **kubeclaw totalt** | **~76h** | |
| KlawAgent MVP (separat) | ~44h | P2 (parallelt) |

---

*Plan: Munin for Erling Kristiansen — April 8, 2026*
