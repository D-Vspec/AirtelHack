# Architecture

Two topologies: the **demo** we run (one docker compose) and the **national-scale** design it is built for. Rendered images: `docs/architecture-demo.png`, `docs/architecture-national.png` (regenerate with `node scripts/render-diagrams.mjs`).

## 1. Demo topology (docker compose)

```mermaid
flowchart LR
  subgraph Clients
    WEB["Web app<br/>Next.js + Tailwind :3000<br/>owner journey + /admin"]
    MOB["Mobile app<br/>Expo React Native"]
  end
  GW["gateway :8080<br/>routing, CORS, rate limit,<br/>admin token, request + correlation IDs"]
  WEB --> GW
  MOB --> GW

  subgraph Services
    ADV["advisor :8090<br/>TypeScript, LangChain.js<br/>DeepSeek / Gemini / Claude / OpenAI"]
    RUL["rules :8082<br/>eligibility, bundles"]
    QUO["quote :8083<br/>pricing, add-ons, accept"]
    REG["registry :8081<br/>GSTIN / Udyam lookup,<br/>checksum, OCR correction"]
    KYC["kyc :8084<br/>consent, OCR pipeline (tesseract),<br/>review queue, resume links,<br/>owner OTP sign-in + KYC reuse"]
    TRK["tracker :8085<br/>order state machine, timeline"]
    NOT["notify :8086<br/>WhatsApp / SMS / email (mock)"]
    SCO["scoring :8087<br/>XGBoost in Go, partner tasks"]
    SIM["simulator<br/>install ops stand-in"]
  end
  GW --> ADV & RUL & QUO & REG & KYC & TRK & NOT & SCO
  ADV -- HTTP --> RUL & QUO & REG
  QUO -- HTTP --> RUL
  KYC -- HTTP --> REG
  SIM -- HTTP --> TRK

  K[("Kafka (KRaft)<br/>topics keyed by leadId<br/>+ dlq.events")]
  ADV -- "lead.created, advisor.profile.updated,<br/>bundle.recommended, lead.unclosed" --> K
  QUO -- "outbox: quote.created / accepted" --> K
  KYC -- "outbox: consent.*, kyc.*, notification.requested" --> K
  TRK -- "outbox: order.*, notification.requested" --> K
  SCO -- "outbox: lead.scored" --> K
  K -- "quote.accepted, kyc.passed, kyc.failed" --> TRK
  K -- "order.*, notification.requested, profile" --> NOT
  K -- "lead.created, profile, lead.unclosed" --> SCO

  PG[("Postgres 16<br/>one schema + one role per service<br/>outbox + processed_events per service<br/>KYC images: deleted at decision or 30 days")]
  REG & RUL & QUO & KYC & TRK & NOT & SCO --- PG
```

Key properties:
- **Decisions are code:** eligibility, pricing and KYC verdicts are deterministic Go. The LLM only converses, extracts a profile, and phrases replies the guard has checked.
- **No lost events:** writes and their events commit together (transactional outbox). Consumers are idempotent (`processed_events`), retry with backoff, then dead-letter. Proven by `make integration` and `make drills`.
- **Owner accounts:** OTP sign-in, order history and KYC reuse live in the kyc service; a reused verification is re-checked against the registry and expires after 365 days.
- **Isolation:** each service owns its schema through its own Postgres role; services talk through Kafka or HTTP only.
- **Graceful degradation:** if rules or quote is down, the advisor keeps everything the owner said, apologises that pricing is briefly unavailable (no product or price is guessed), and quotes when they try again.
- **Observability:** `/metrics` on every service, JSON logs with request and correlation IDs, and an admin status page.

## 2. National-scale topology

```mermaid
flowchart TB
  subgraph Edge["Edge, per region (North, South, East, West)"]
    CDN["CDN: web app, fonts per script"]
    WA["WhatsApp Business API / SMS gateway"]
    RGW["Regional gateways<br/>autoscaled, WAF, rate limits, SSO for back office"]
  end
  OWN["Owners: phone web, Expo app, WhatsApp"] --> CDN & WA
  CDN --> RGW
  WA --> RGW
  PART["Field sales and channel partners<br/>back office, tablets"] --> RGW

  subgraph Region["Each region: Kubernetes cluster"]
    SVC["Stateless services, horizontally scaled<br/>advisor, rules, quote, registry, kyc, tracker, notify, scoring"]
    OCR["KYC OCR worker pool<br/>(queue-driven, CPU autoscaling)"]
    CACHE["Registry cache<br/>(GSTN / Udyam APIs behind it)"]
  end
  RGW --> SVC
  SVC --> OCR
  SVC --> CACHE

  subgraph Data["Data plane"]
    KAF[("Kafka, multi-AZ per region<br/>partitioned by leadId (ordering per owner)<br/>MirrorMaker to the analytics region")]
    DB[("Postgres 16 per region, primary + replicas<br/>rows homed by owner region (data residency)<br/>one schema + role per service")]
    OBJ[("Object storage, encrypted<br/>KYC images, 30-day lifecycle")]
  end
  SVC --> KAF
  SVC --> DB
  OCR --> OBJ

  subgraph Platform
    LLM["LLM providers behind one adapter<br/>eval-gated per language, failover"]
    OBS["Prometheus + Grafana, log pipeline<br/>correlation IDs end to end"]
    ML["Offline training: lead score<br/>time-split eval, model registry"]
  end
  SVC --> LLM
  SVC -.-> OBS
  KAF --> ML
  ML -. "model.json" .-> SVC
```

What changes from the demo:
- **Data:** Postgres 16 in each region (primary plus read replicas); each owner's rows live in their home region. Same engine as the demo, so no migration.
- **Kafka:** partitioned by `leadId`, so one owner's events stay in order while partitions scale out.
- **Gateways:** per region, with WAF and back-office SSO.
- **KYC:** OCR moves to a worker pool, and images move to encrypted object storage with lifecycle deletion.
- **Notifications:** real WhatsApp and SMS adapters replace the mocks behind the same interface.
- **Model:** the scoring model is retrained offline and shipped as `model.json` through a registry.

The service code and contracts stay the same.
