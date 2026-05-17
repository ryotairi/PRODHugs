#!/usr/bin/env bash
# Smoke-test the backend end-to-end with a few curl calls against a freshly
# built binary running on top of a freshly migrated Postgres. We are NOT
# trying to be exhaustive — we just want to know "the service starts and
# the happy path through register / login / /users/me / a v2 endpoint
# works". Anything more thorough belongs in real Go tests once those
# arrive.
#
# Outputs lines in the format "OK  <name>" / "FAIL <name>: <reason>" so
# summarize.sh can count failures. Exits non-zero if anything failed.
#
# Required env:
#   POSTGRES_URL                — connection string for an empty/migrated DB
#   JWT_SECRET                  — backend's signing key
#   BACKEND_BIN                 — path to the built `service` binary
#   PORT (optional, default 8080)

set -uo pipefail

PORT="${PORT:-8080}"
BASE="http://127.0.0.1:${PORT}"
COOKIE_JAR="$(mktemp)"
trap 'rm -f "$COOKIE_JAR"' EXIT

fails=0
report() {
    local status="$1" name="$2" detail="${3:-}"
    if [ -n "$detail" ]; then
        echo "${status} ${name}: ${detail}"
    else
        echo "${status} ${name}"
    fi
    [ "$status" = "FAIL" ] && fails=$((fails + 1))
}

# Run the backend in the background. We redirect logs so a failed run still
# leaves something useful in $SERVICE_LOG.
SERVICE_LOG="$(mktemp)"
SERVER_ADDR="0.0.0.0:${PORT}" \
    METRICS_ADDR=":0" \
    POSTGRES_URL="$POSTGRES_URL" \
    POSTGRES_MAX_CONNS=10 \
    JWT_SECRET="$JWT_SECRET" \
    JWT_COOKIE_SECURE=false \
    CORS_ALLOW_ORIGINS="http://localhost:3000" \
    TELEGRAM_BOT_TOKEN="" \
    TELEGRAM_BOT_USERNAME="prodhugs_smoke_bot" \
    "$BACKEND_BIN" >"$SERVICE_LOG" 2>&1 &
SERVICE_PID=$!
trap 'kill "$SERVICE_PID" 2>/dev/null || true; rm -f "$COOKIE_JAR"' EXIT

# Wait for /ping. Generous budget — Go's cold start + migrations can take
# 5+ seconds the first time.
ready=0
for i in $(seq 1 30); do
    if curl -sf -o /dev/null "$BASE/api/v1/ping"; then
        ready=1
        break
    fi
    sleep 1
done
if [ "$ready" -ne 1 ]; then
    report FAIL "service start" "не ответил на /api/v1/ping за 30 секунд"
    echo "--- service log (tail) ---"
    tail -n 60 "$SERVICE_LOG"
    exit 1
fi
report OK "service start"

# ── /api/v1/ping anonymous ────────────────────────────────────────────────
resp=$(curl -sf "$BASE/api/v1/ping" 2>&1) && \
    [[ "$resp" == *"PONG_PUBLIC"* ]] && \
    report OK "GET /api/v1/ping" || \
    report FAIL "GET /api/v1/ping" "ответ=${resp:-empty}"

# Pick a username unique to this run so reruns don't collide.
USER="smoke_$(date +%s)_$RANDOM"
PASS="Smoke_test_password_42!"

# ── Register ──────────────────────────────────────────────────────────────
reg=$(curl -s -o /tmp/reg.json -w '%{http_code}' \
    -c "$COOKIE_JAR" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"${USER}\",\"password\":\"${PASS}\"}" \
    "$BASE/api/v1/auth/register")
if [ "$reg" = "200" ] || [ "$reg" = "201" ]; then
    report OK "POST /auth/register"
else
    report FAIL "POST /auth/register" "HTTP $reg, body=$(head -c 200 /tmp/reg.json)"
fi

TOKEN=$(jq -r '.token // empty' /tmp/reg.json 2>/dev/null || echo "")
if [ -z "$TOKEN" ]; then
    # Try login as a fallback (in case register returned without token).
    log=$(curl -s -o /tmp/login.json -w '%{http_code}' \
        -c "$COOKIE_JAR" \
        -H 'Content-Type: application/json' \
        -d "{\"username\":\"${USER}\",\"password\":\"${PASS}\"}" \
        "$BASE/api/v1/auth/login")
    if [ "$log" = "200" ]; then
        TOKEN=$(jq -r '.token // empty' /tmp/login.json)
        report OK "POST /auth/login"
    else
        report FAIL "POST /auth/login" "HTTP $log"
    fi
fi

auth=( -H "Authorization: Bearer $TOKEN" )

if [ -z "$TOKEN" ]; then
    report FAIL "auth flow" "не получили access-токен — дальнейшие проверки пропускаю"
    exit "$fails"
fi

# ── /api/v1/users/me ──────────────────────────────────────────────────────
me=$(curl -s -o /tmp/me.json -w '%{http_code}' "${auth[@]}" "$BASE/api/v1/users/me")
if [ "$me" = "200" ] && [ "$(jq -r '.username // empty' /tmp/me.json)" = "$USER" ]; then
    report OK "GET /users/me"
else
    report FAIL "GET /users/me" "HTTP $me, username=$(jq -r '.username // empty' /tmp/me.json)"
fi

# ── /api/v2/users/search?q=smoke ──────────────────────────────────────────
search=$(curl -s -o /tmp/search.json -w '%{http_code}' "${auth[@]}" \
    "$BASE/api/v2/users/search?q=smoke")
if [ "$search" = "200" ] && jq -e '. | type == "array"' /tmp/search.json >/dev/null; then
    report OK "GET /api/v2/users/search"
else
    report FAIL "GET /api/v2/users/search" "HTTP $search"
fi

# ── /api/v2/users/@smoke_xxx/profile (own profile by username) ────────────
prof=$(curl -s -o /tmp/prof.json -w '%{http_code}' "${auth[@]}" \
    "$BASE/api/v2/users/@${USER}/profile")
if [ "$prof" = "200" ] && [ "$(jq -r '.username // empty' /tmp/prof.json)" = "$USER" ]; then
    report OK "GET /api/v2/users/@<name>/profile"
else
    report FAIL "GET /api/v2/users/@<name>/profile" "HTTP $prof"
fi

# ── /api/v2/daily-reward/status ───────────────────────────────────────────
drs=$(curl -s -o /tmp/drs.json -w '%{http_code}' "${auth[@]}" \
    "$BASE/api/v2/daily-reward/status")
if [ "$drs" = "200" ] && jq -e 'has("can_claim") and has("next_claim_at")' /tmp/drs.json >/dev/null; then
    report OK "GET /api/v2/daily-reward/status"
else
    report FAIL "GET /api/v2/daily-reward/status" "HTTP $drs"
fi

# ── /api/v1/balance ───────────────────────────────────────────────────────
bal=$(curl -s -o /tmp/bal.json -w '%{http_code}' "${auth[@]}" "$BASE/api/v1/balance")
if [ "$bal" = "200" ] && jq -e 'has("amount")' /tmp/bal.json >/dev/null; then
    report OK "GET /balance"
else
    report FAIL "GET /balance" "HTTP $bal"
fi

# ── POST /daily-reward (claim) — should succeed first time ────────────────
claim=$(curl -s -o /tmp/claim.json -w '%{http_code}' -X POST "${auth[@]}" \
    "$BASE/api/v1/daily-reward")
if [ "$claim" = "200" ] && jq -e 'has("amount")' /tmp/claim.json >/dev/null; then
    report OK "POST /daily-reward"
else
    report FAIL "POST /daily-reward" "HTTP $claim"
fi

# If anything failed, dump the tail of the server log so the report has
# something to point at.
if [ "$fails" -gt 0 ]; then
    echo
    echo "--- service log (last 60 lines) ---"
    tail -n 60 "$SERVICE_LOG"
fi

exit "$fails"
