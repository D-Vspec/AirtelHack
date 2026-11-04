# Failure drills

`make drills` (go/integration/drills_test.go) runs all three drills against the live stack using `docker compose stop/start`. Each one fails if the behaviour below does not happen. The timings were measured on the build machine (Docker Desktop on Windows) on 2026-11-05.

| Drill | What we did | What the owner and the system saw | Recovery |
|---|---|---|---|
| **Kafka down** | Stopped Kafka, then accepted a quote and ran a KYC check | Both succeeded (quote 200, KYC pass in 2.3 s). Each write and its event commit together into the service's outbox. `/metrics` showed the backlog: `outbox_pending` quote = 1, kyc = 3. Nothing was lost or blocked. | Started Kafka. The relays drained both outboxes, and the tracker caught up to `kyc_passed` **36 s** after start (most of that is Kafka's own startup). |
| **kyc service down** | Stopped kyc | KYC routes answer **503 `UPSTREAM_UNAVAILABLE`** with `Retry-After: 5` (JSON, not an empty 502). `/api/health` = degraded, kyc = down. Quotes, orders and the rest of the journey keep working: a new order was created while kyc was down. | Started kyc. It served again **3 s** later, and the order waiting in `kyc_pending` moved on once KYC passed. |
| **Postgres restart** | Stopped Postgres | Requests that need the database fail with **500 in about 7 s** (the connection timeout); none hung and no service crashed. | Started Postgres. The services' pgx pools reconnected without restarts: the first DB read succeeded **1 s** later, and a new quote → order flowed end to end **5 s** after the restart. |

Also covered by `make integration`:
- Tracker and notify stopped while a quote is accepted and KYC passes: after they restart, the order reaches `activated` and every milestone message is sent exactly once.
- A malformed event is dead-lettered to `dlq.events` after retries, and the consumer keeps going.

What would improve the drills:
- The Postgres outage costs about 7 s per request before failing. A shorter connect timeout plus a circuit breaker would fail in under a second.
- The web app shows the gateway's 503 message but does not retry by itself.
- The advisor publishes its events directly (no outbox), so an advisor event emitted during a Kafka outage is lost. Profile updates are re-sent on the next turn, but `lead.created` is not.
