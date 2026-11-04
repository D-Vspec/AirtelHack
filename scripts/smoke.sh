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
  curl -fsS "$GW/api/notify/pings" | grep -q "$ID" && { echo "   event $ID consumed and stored"; break; }
  [ "$i" = 20 ] && { echo "SMOKE FAILED: event $ID was not consumed within 20s (check: docker compose logs notify kafka)"; exit 1; }
  sleep 1
done

# Registry lookup needs `make seed`; an unseeded database answers 503 and is skipped, not failed.
REG=$(curl -s -w ' %{http_code}' "$GW/api/registry/gstin/37IHKXV5167L1ZS")
case "$REG" in
  *' 503') echo "   registry not seeded (run make seed); lookup check skipped" ;;
  *'"status":"Cancelled"'*' 200') echo "   registry lookup ok" ;;
  *) echo "SMOKE FAILED: registry lookup: $REG"; exit 1 ;;
esac

# KYC: uploads are refused without consent; with consent, clean specimen K03 passes (needs make seed).
DOCS="$(cd "$(dirname "$0")/.." && pwd)/data/kyc_documents"
LEAD="smoke-kyc-$(date +%s)"
CODE=$(curl -s -o /dev/null -w '%{http_code}' -X PUT --data-binary @"$DOCS/K03_gst_certificate.png" "$GW/api/kyc/leads/$LEAD/documents/gst_certificate")
[ "$CODE" = 403 ] || { echo "SMOKE FAILED: KYC upload without consent answered $CODE, want 403"; exit 1; }
curl -fsS -X POST "$GW/api/kyc/consents" -H 'Content-Type: application/json' -d "{\"leadId\":\"$LEAD\",\"purposes\":[\"kyc_verification\"],\"version\":\"v1\"}" >/dev/null
for d in gst_certificate udyam_certificate address_proof signatory_id; do
  curl -fsS -X PUT --data-binary @"$DOCS/K03_$d.png" "$GW/api/kyc/leads/$LEAD/documents/$d" >/dev/null || { echo "SMOKE FAILED: KYC upload $d"; exit 1; }
done
KYC=$(curl -s -w ' %{http_code}' -X POST "$GW/api/kyc/leads/$LEAD/check")
case "$KYC" in
  *' 503') echo "   KYC check skipped (registry not seeded)" ;;
  *'"route":"pass"'*' 200') echo "   KYC consent, upload and check ok (K03 pass)" ;;
  *) echo "SMOKE FAILED: KYC check: $KYC"; exit 1 ;;
esac
echo "SMOKE OK"
