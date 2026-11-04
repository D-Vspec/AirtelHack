Goal: make it judge-proof and ship the deliverables.
1. `make scenarios` replays 25+ user-story scenarios against the live stack and prints pass or fail.
2. Failure drills: kill Kafka, kill kyc, restart Postgres, and show graceful behaviour.
3. Observability: structured logs carrying lead and correlation IDs, /metrics on every service, and a simple status page in admin.
4. Security pass: input validation, gateway rate limiting, upload size and type limits, PII masking, dependency audit, and per-service Postgres roles so no service can read another's schema.
5. Deliverables: architecture diagram (Mermaid plus exported PNG) showing the demo topology and the national-scale topology; business case content for 10 slides or fewer in docs/business-case.md using numbers from docs/analytics.md (segment priority, bundles and pricing, rollout, field and partner role, KPIs with targets, risks); 1-page privacy and security note; 3-minute demo script with shot list; README quickstart verified from a clean clone.
Done when: a fresh clone runs `make up seed smoke scenarios` successfully and every deliverable file exists.
