#!/usr/bin/env bash
# Phase 0 smoke test: every service healthy, and one event travels
# gateway -> Kafka -> notify consumer -> Postgres -> HTTP.
set -euo pipefail
GW="${GATEWAY_URL:-http://localhost:8080}"

echo "1/3 waiting for gateway..."
for i in $(seq 1 60); do curl -fsS "$GW/healthz" >/dev/null 2>&1 && break; sleep 2; [ "$i" = 60 ] && { echo "gateway did not come up"; exit 1; }; done

echo "2/3 checking every service..."
for i in $(seq 1 30); do
  H=$(curl -fsS "$GW/api/health" || true)
  echo "$H" | grep -q '"status":"ok"' && { echo "   all services ok"; break; }
  sleep 2
  [ "$i" = 30 ] && { echo "services not healthy: $H"; exit 1; }
done

echo "3/3 sending ping through Kafka..."
MSG="smoke-$(date +%s)"
ID=$(curl -fsS -X POST "$GW/api/system/ping" -H 'Content-Type: application/json' -d "{\"message\":\"$MSG\"}" | sed -E 's/.*"id":"([^"]+)".*/\1/')
[ -n "$ID" ] || { echo "no event id returned"; exit 1; }
for i in $(seq 1 20); do
  curl -fsS "$GW/api/notify/pings" | grep -q "$ID" && { echo "SMOKE OK: event $ID consumed and stored"; exit 0; }
  sleep 1
done
echo "SMOKE FAILED: event $ID was not consumed within 20s (check: docker compose logs notify kafka)"; exit 1
