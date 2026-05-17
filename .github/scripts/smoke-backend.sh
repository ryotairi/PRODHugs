#!/usr/bin/env bash
# Smoke-test the backend end-to-end with curl. Output lines:
#   OK <name>
#   FAIL <name>: <reason>
# Plus a SUMMARY footer at the end. summarize.sh counts OK/FAIL lines for
# the metric column; the script's own exit code is the number of failures.
#
# Required env:
#   POSTGRES_URL  — connection string for a freshly migrated DB
#   JWT_SECRET    — backend's signing key
#   BACKEND_BIN   — path to the built `service` binary
#   PORT (optional, default 8080)

set -uo pipefail

PORT="${PORT:-8080}"
BASE="http://127.0.0.1:${PORT}"

COOKIE_JAR="$(mktemp)"
SERVICE_LOG="$(mktemp)"

cleanup() {
    if [ -n "${SERVICE_PID:-}" ]; then
        kill "$SERVICE_PID" 2>/dev/null || true
    fi
    rm -f "$COOKIE_JAR"
}
trap cleanup EXIT

fails=0
ok_count=0
report() {
    local status="$1" name="$2" detail="${3:-}"
    if [ -n "$detail" ]; then
        echo "${status} ${name}: ${detail}"
    else
        echo "${status} ${name}"
    fi
    case "$status" in
        OK)   ok_count=$((ok_count + 1)) ;;
        FAIL) fails=$((fails + 1)) ;;
    esac
}

# httpcode <varname> <url> [extra curl args...]
# Performs a request, stashes the response body to a temp file, and assigns
# the HTTP status code to <varname>. Body file path goes to ${varname}_body.
http_call() {
    local var="$1"; shift
    local url="$1"; shift
    local body
    body=$(mktemp)
    local code
    code=$(curl -sS -o "$body" -w '%{http_code}' "$@" "$url" 2>/dev/null || echo "000")
    eval "${var}=\"\${code}\""
    eval "${var}_body=\"\${body}\""
}

# ── Boot the service ──────────────────────────────────────────────────────
SERVER_ADDR="0.0.0.0:${PORT}" \
    METRICS_ADDR=":0" \
    POSTGRES_URL="$POSTGRES_URL" \
    POSTGRES_MAX_CONNS=10 \
    JWT_SECRET="$JWT_SECRET" \
    JWT_COOKIE_SECURE=false \
    CORS_ALLOW_ORIGINS="http://localhost:3000" \
    TELEGRAM_BOT_TOKEN="" \
    TELEGRAM_BOT_USERNAME="prodhugs_smoke_bot" \
    AUTH_RATE_LIMIT_DISABLED="${AUTH_RATE_LIMIT_DISABLED:-true}" \
    "$BACKEND_BIN" >"$SERVICE_LOG" 2>&1 &
SERVICE_PID=$!

ready=0
for _ in $(seq 1 30); do
    if curl -sf -o /dev/null "$BASE/api/v1/ping"; then
        ready=1
        break
    fi
    sleep 1
done
if [ "$ready" -ne 1 ]; then
    report FAIL "service start" "не ответил на /api/v1/ping за 30 секунд"
    echo
    echo "--- service log (tail) ---"
    tail -n 60 "$SERVICE_LOG"
    echo
    echo "SUMMARY: 0 OK, 1 FAIL"
    exit 1
fi
report OK "service start"

# ── /api/v1/ping anonymous ────────────────────────────────────────────────
http_call resp "$BASE/api/v1/ping"
if [ "$resp" = "200" ] && grep -q 'PONG_PUBLIC' "$resp_body" 2>/dev/null; then
    report OK "GET /api/v1/ping"
else
    report FAIL "GET /api/v1/ping" "HTTP $resp, body=$(head -c 200 "$resp_body" 2>/dev/null || echo none)"
fi

# Unique-per-run usernames so reruns don't trip on the previous run's data.
suffix="$(date +%s)_${RANDOM:-$$}"
USER_A="smoke_a_${suffix}"
USER_B="smoke_b_${suffix}"
PASS="Smoke_test_password_42!"

# ── Negative: register with a weak password should be rejected ────────────
http_call reg_weak "$BASE/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"smoke_weak_${suffix}\",\"password\":\"abc\"}"
if [ "$reg_weak" = "400" ] || [ "$reg_weak" = "422" ]; then
    report OK "register rejects weak password"
else
    report FAIL "register rejects weak password" "HTTP $reg_weak (ожидали 400/422)"
fi

# ── Auth-required endpoint without a token should be 401 ──────────────────
http_call no_auth "$BASE/api/v1/users/me"
if [ "$no_auth" = "401" ]; then
    report OK "users/me without token rejects (401)"
else
    report FAIL "users/me without token rejects (401)" "HTTP $no_auth"
fi

# ── Register user A ───────────────────────────────────────────────────────
http_call reg_a "$BASE/api/v1/auth/register" \
    -c "$COOKIE_JAR" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"${USER_A}\",\"password\":\"${PASS}\"}"
if [ "$reg_a" = "200" ] || [ "$reg_a" = "201" ]; then
    report OK "POST /auth/register (user A)"
else
    report FAIL "POST /auth/register (user A)" "HTTP $reg_a, body=$(head -c 200 "$reg_a_body")"
fi

TOKEN_A=$(jq -r '.token // empty' "$reg_a_body" 2>/dev/null || echo "")
USER_A_ID=$(jq -r '.user.id // empty' "$reg_a_body" 2>/dev/null || echo "")
if [ -z "$TOKEN_A" ]; then
    # Login fallback in case the register endpoint stopped returning the token.
    http_call log_a "$BASE/api/v1/auth/login" \
        -c "$COOKIE_JAR" \
        -H 'Content-Type: application/json' \
        -d "{\"username\":\"${USER_A}\",\"password\":\"${PASS}\"}"
    if [ "$log_a" = "200" ]; then
        TOKEN_A=$(jq -r '.token // empty' "$log_a_body")
        USER_A_ID=$(jq -r '.user.id // empty' "$log_a_body")
        report OK "POST /auth/login (user A fallback)"
    else
        report FAIL "POST /auth/login (user A fallback)" "HTTP $log_a"
    fi
fi

if [ -z "$TOKEN_A" ]; then
    report FAIL "auth flow user A" "не получили токен — пропускаю авторизованные проверки"
    echo
    echo "--- service log (tail) ---"
    tail -n 60 "$SERVICE_LOG"
    echo
    echo "SUMMARY: ${ok_count} OK, ${fails} FAIL"
    exit "$fails"
fi
auth_a=( -H "Authorization: Bearer $TOKEN_A" )

# ── Negative: registering the same username again is a duplicate ──────────
http_call reg_dup "$BASE/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"${USER_A}\",\"password\":\"${PASS}\"}"
if [ "$reg_dup" = "409" ] || [ "$reg_dup" = "400" ]; then
    report OK "register rejects duplicate username"
else
    report FAIL "register rejects duplicate username" "HTTP $reg_dup (ожидали 409/400)"
fi

# ── Register user B (target for hug + note flows) ─────────────────────────
http_call reg_b "$BASE/api/v1/auth/register" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"${USER_B}\",\"password\":\"${PASS}\"}"
if [ "$reg_b" = "200" ] || [ "$reg_b" = "201" ]; then
    report OK "POST /auth/register (user B)"
else
    report FAIL "POST /auth/register (user B)" "HTTP $reg_b"
fi
USER_B_ID=$(jq -r '.user.id // empty' "$reg_b_body" 2>/dev/null || echo "")

# ── /api/v1/users/me ──────────────────────────────────────────────────────
http_call me "$BASE/api/v1/users/me" "${auth_a[@]}"
if [ "$me" = "200" ] && [ "$(jq -r '.username // empty' "$me_body")" = "$USER_A" ]; then
    report OK "GET /users/me"
else
    report FAIL "GET /users/me" "HTTP $me, username=$(jq -r '.username // empty' "$me_body")"
fi

# ── /api/v2/users/search with bare query ──────────────────────────────────
http_call search "$BASE/api/v2/users/search?q=smoke" "${auth_a[@]}"
if [ "$search" = "200" ] && jq -e '. | type == "array"' "$search_body" >/dev/null; then
    report OK "GET /api/v2/users/search?q=smoke"
else
    report FAIL "GET /api/v2/users/search?q=smoke" "HTTP $search"
fi

# ── /api/v2/users/search with @username prefix ────────────────────────────
http_call search_at "$BASE/api/v2/users/search?q=%40${USER_B}" "${auth_a[@]}"
if [ "$search_at" = "200" ] && jq -e --arg u "$USER_B" 'any(.[]; .username == $u)' "$search_at_body" >/dev/null; then
    report OK "GET /api/v2/users/search?q=@..."
else
    report FAIL "GET /api/v2/users/search?q=@..." "HTTP $search_at"
fi

# ── /api/v2/users/@USER_B/profile ─────────────────────────────────────────
http_call prof "$BASE/api/v2/users/@${USER_B}/profile" "${auth_a[@]}"
if [ "$prof" = "200" ] && [ "$(jq -r '.username // empty' "$prof_body")" = "$USER_B" ]; then
    report OK "GET /api/v2/users/@<name>/profile"
else
    report FAIL "GET /api/v2/users/@<name>/profile" "HTTP $prof"
fi

# ── /api/v2/daily-reward/status ───────────────────────────────────────────
http_call drs "$BASE/api/v2/daily-reward/status" "${auth_a[@]}"
if [ "$drs" = "200" ] && jq -e 'has("can_claim") and has("next_claim_at")' "$drs_body" >/dev/null; then
    report OK "GET /api/v2/daily-reward/status"
else
    report FAIL "GET /api/v2/daily-reward/status" "HTTP $drs"
fi

# ── Note CRUD on user B ───────────────────────────────────────────────────
http_call note_put "$BASE/api/v2/users/@${USER_B}/note" "${auth_a[@]}" \
    -X PUT \
    -H 'Content-Type: application/json' \
    -d '{"content":"smoke-test note ✓"}'
if [ "$note_put" = "200" ]; then
    report OK "PUT /api/v2/users/@<name>/note"
else
    report FAIL "PUT /api/v2/users/@<name>/note" "HTTP $note_put, body=$(head -c 200 "$note_put_body")"
fi

http_call note_get "$BASE/api/v2/users/@${USER_B}/note" "${auth_a[@]}"
if [ "$note_get" = "200" ] && jq -e '.content == "smoke-test note ✓"' "$note_get_body" >/dev/null; then
    report OK "GET /api/v2/users/@<name>/note"
else
    report FAIL "GET /api/v2/users/@<name>/note" "HTTP $note_get"
fi

http_call notes_list "$BASE/api/v2/notes" "${auth_a[@]}"
if [ "$notes_list" = "200" ] && jq -e '. | type == "array" and length >= 1' "$notes_list_body" >/dev/null; then
    report OK "GET /api/v2/notes"
else
    report FAIL "GET /api/v2/notes" "HTTP $notes_list"
fi

http_call note_del "$BASE/api/v2/users/@${USER_B}/note" "${auth_a[@]}" -X DELETE
if [ "$note_del" = "204" ]; then
    report OK "DELETE /api/v2/users/@<name>/note"
else
    report FAIL "DELETE /api/v2/users/@<name>/note" "HTTP $note_del"
fi

# Confirm the note is really gone — should be 404 now.
http_call note_get_gone "$BASE/api/v2/users/@${USER_B}/note" "${auth_a[@]}"
if [ "$note_get_gone" = "404" ]; then
    report OK "GET /note after delete returns 404"
else
    report FAIL "GET /note after delete returns 404" "HTTP $note_get_gone (ожидали 404)"
fi

# ── Balance + daily reward (A) ────────────────────────────────────────────
http_call bal "$BASE/api/v1/balance" "${auth_a[@]}"
if [ "$bal" = "200" ] && jq -e 'has("amount")' "$bal_body" >/dev/null; then
    report OK "GET /balance"
else
    report FAIL "GET /balance" "HTTP $bal"
fi

http_call claim "$BASE/api/v1/daily-reward" "${auth_a[@]}" -X POST
if [ "$claim" = "200" ] && jq -e 'has("amount")' "$claim_body" >/dev/null; then
    report OK "POST /daily-reward (first claim)"
else
    report FAIL "POST /daily-reward (first claim)" "HTTP $claim"
fi

# Second claim same day — backend returns 200 with already_claimed=true.
http_call claim2 "$BASE/api/v1/daily-reward" "${auth_a[@]}" -X POST
if [ "$claim2" = "200" ] && jq -e '.already_claimed == true' "$claim2_body" >/dev/null; then
    report OK "POST /daily-reward (idempotent second call)"
else
    report FAIL "POST /daily-reward (idempotent second call)" "HTTP $claim2, body=$(head -c 200 "$claim2_body")"
fi

# ── Cooldown lookup between A and B (intimacy-aware) ──────────────────────
if [ -n "$USER_B_ID" ]; then
    http_call cd "$BASE/api/v1/hugs/cooldown/${USER_B_ID}" "${auth_a[@]}"
    if [ "$cd" = "200" ] && jq -e 'has("cooldown_seconds")' "$cd_body" >/dev/null; then
        report OK "GET /hugs/cooldown/<id>"
    else
        report FAIL "GET /hugs/cooldown/<id>" "HTTP $cd"
    fi
fi

# ── Final accounting ──────────────────────────────────────────────────────
echo
echo "SUMMARY: ${ok_count} OK, ${fails} FAIL"

if [ "$fails" -gt 0 ]; then
    echo
    echo "--- service log (last 80 lines) ---"
    tail -n 80 "$SERVICE_LOG"
fi

exit "$fails"
