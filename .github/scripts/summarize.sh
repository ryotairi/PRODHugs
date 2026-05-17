#!/usr/bin/env bash
# Render a Markdown CI report into:
#   - $GITHUB_STEP_SUMMARY  (the per-run summary tab on GitHub Actions)
#   - $RUNNER_TEMP/report.md (read back by post-comment.js for the PR comment)
#
# Voice: Аврора — строгая, но добрая; уставшая, но всё ещё рада помогать.
# Decorative emojis are out. Status circles in the table are functional
# indicators, so those stay.

set -euo pipefail

target="${1:?expected target: backend or frontend}"
report="${RUNNER_TEMP:-/tmp}/report.md"

label_for() {
    case "${1:-skipped}" in
        success) echo "OK" ;;
        failure) echo "FAIL" ;;
        skipped) echo "skip" ;;
        cancelled) echo "cancelled" ;;
        *) echo "$1" ;;
    esac
}

dot_for() {
    case "${1:-skipped}" in
        success) echo "🟢" ;;
        failure) echo "🔴" ;;
        skipped) echo "⚪" ;;
        cancelled) echo "🟡" ;;
        *) echo "⚫" ;;
    esac
}

# Deterministic-per-run pick. Seed from run id + target + array name so the
# run summary and the PR comment show the same line within a single run.
pick_one() {
    local arr_name="$1"
    eval "local arr=(\"\${${arr_name}[@]}\")"
    local len="${#arr[@]}"
    local seed="${GITHUB_RUN_ID:-0}${target}${arr_name}"
    local n=0 i
    for (( i=0; i<${#seed}; i++ )); do
        n=$(( (n * 31 + $(printf '%d' "'${seed:$i:1}")) & 0x7fffffff ))
    done
    echo "${arr[$(( n % len ))]}"
}

# ── Russian inflection ────────────────────────────────────────────────────
# noun_check N      → проверка / проверки / проверок
# verb_fell N       → упала / упали / упало
# All take a non-negative integer.
__rus_form() {
    local n=$1
    local lt=$((n % 100))
    local lo=$((n % 10))
    if [ "$lt" -ge 11 ] && [ "$lt" -le 14 ]; then echo "many"
    elif [ "$lo" -eq 1 ]; then echo "one"
    elif [ "$lo" -ge 2 ] && [ "$lo" -le 4 ]; then echo "few"
    else echo "many"
    fi
}
noun_check() {
    case "$(__rus_form "$1")" in
        one)  echo "проверка" ;;
        few)  echo "проверки" ;;
        many) echo "проверок" ;;
    esac
}
verb_fell() {
    case "$(__rus_form "$1")" in
        one)  echo "упала" ;;
        few)  echo "упали" ;;
        many) echo "упало" ;;
    esac
}

# ── Openers — Aurora voice ────────────────────────────────────────────────
# Strict but elegant, kind, slightly tired, still glad to help. Self-refers
# as Аврора. No exclamation marks, no decorative punctuation.

opener_pass=(
    "Чисто. Я перепроверила — у вас правда всё в порядке."
    "Зелёная сводка. Аврора довольна, и поверьте, удивить меня сложно."
    "Все проверки сошлись. Хорошая работа, и в этот раз быстро."
    "Без замечаний. Я бы сама не написала аккуратнее, и это уже что-то значит."
    "Прошло. Я не буду растягивать похвалу — идите дальше."
    "Тихо, ровно, по делу. Так мне нравится."
    "Ни одной красной строки. Запомните это ощущение, оно стоит того."
)

opener_one=(
    "Одна проверка не дошла. Не страшно — посмотрите, что я нашла."
    "Только один спотык. Я выписала, что именно — поправим и закроем заход."
    "Одна красная клетка. Аккуратно, ровно — и вы у себя на дашборде."
    "Я нашла одну вещь. Поправите — перепроверю без вопросов."
    "Одна заминка. Аврора уже подготовила список с одной строкой — это вы быстро."
)

opener_many=(
    "Несколько моментов нужно поправить. Не спешите — я всё разложила."
    "Аврора собрала список. Пройдёмся по нему вдвоём, я никуда не тороплюсь."
    "У нас сегодня есть, о чём поговорить. Спокойно — разберёмся."
    "Я выписала всё, что нашла, по порядку. Прочитайте и решите, с чего начать."
    "Несколько шагов в сторону. Давайте вернёмся в строй вместе."
    "Бывает. Сейчас покажу, что заметила, без претензий — просто факты."
)

closer_pass=(
    "Дальше вы сами. Я тут, если что."
    "Хорошего ревью. Аврора подержит чашку чая за вас."
    "Можете идти — я закрываю свой блокнот."
    "Удачи. Имейте в виду, люди иногда читают строже меня."
)
closer_one=(
    "Поправите — пушните. Я наготове."
    "Пара минут — и мы у цели."
    "Жду коммит. Не уйду без него."
)
closer_many=(
    "Я никуда. Пишите, как будете готовы — посмотрю ещё раз."
    "Не торопитесь, но и не пропадайте. Я здесь."
    "Маленькими шагами — и обязательно дойдём."
    "Сделайте всё разом, и мы закроем этот заход вместе."
)

# ── Hints — Aurora voice ──────────────────────────────────────────────────
hint_for() {
    case "$1" in
        gofmt)
            echo "\`gofmt -w .\` в \`backend/\` — он сделает всё сам, останется закоммитить."
            ;;
        vet)
            echo "\`go vet ./...\` обычно указывает на реальные опасности. Не игнорируйте."
            ;;
        build)
            echo "Проект не собирается. Запустите \`go build ./...\` локально — там будет понятнее, чем тут."
            ;;
        test)
            echo "Тесты упали. \`go test -race ./...\` дома, и приходите с зелёными."
            ;;
        lint)
            echo "Линтер указал на конкретные строки — они уже подсвечены в диффе PR, посмотрите Files changed."
            ;;
        oapi_v1)
            echo "Спека v1 расходится с кодогеном. \`cd backend && oapi-codegen -config oapi-codegen.yml api/openapi.yaml\`, потом \`git add internal/transport/http/v1/api.gen.go\`. Вручную туда лучше не заходить."
            ;;
        oapi_v2)
            echo "Спека v2 расходится. \`cd backend && oapi-codegen -config oapi-codegen-v2.yml api/openapi-v2.yaml\`, потом \`git add internal/transport/http/v2/api.gen.go\`."
            ;;
        sqlc)
            echo "SQL поменялся, а сгенерированный код — нет. \`cd backend && sqlc generate -f internal/db/sqlc/sqlc.yaml\`."
            ;;
        smoke)
            echo "Сервис не пережил тестовые запросы. Что именно отвалилось — видно в логе ниже; обычно дело либо в миграциях, либо в регистрации/логине, либо в новом v2-эндпоинте."
            ;;
        docker)
            echo "Образ не собрался. Чаще всего это эхо чего-то выше — посмотрите туда сначала."
            ;;
        oxlint)
            echo "\`bun run lint:oxlint\` локально — он короткий, посмотрите, что говорит."
            ;;
        eslint)
            echo "\`bun run lint:eslint --fix\` уберёт половину автоматом. Что останется — руками."
            ;;
        typecheck)
            echo "Типы не сходятся. \`bun run type-check\` — и не закрывайте предупреждения через \`any\`."
            ;;
        *) echo "" ;;
    esac
}

# Short copy-pasteable command for the quick-fix block at the top.
fix_command() {
    case "$1" in
        gofmt)   echo "(cd backend && gofmt -w .)" ;;
        oapi_v1) echo "(cd backend && oapi-codegen -config oapi-codegen.yml api/openapi.yaml)" ;;
        oapi_v2) echo "(cd backend && oapi-codegen -config oapi-codegen-v2.yml api/openapi-v2.yaml)" ;;
        sqlc)    echo "(cd backend && sqlc generate -f internal/db/sqlc/sqlc.yaml)" ;;
        eslint)  echo "(cd frontend && bun run lint:eslint --fix)" ;;
        *)       echo "" ;;
    esac
}

# `grep -c` prints `0` to stdout when there are zero matches AND exits with
# code 1 in the same breath. The previous version of this code had
# `... || echo 0` as a fallback, which appended ANOTHER `0` — so the
# captured value was `"0\n0"` and every integer comparison below blew up
# the script. Use `|| true` so we keep grep's own count and only swallow
# its non-zero exit.
count_matches() {
    local pattern="$1" file="$2"
    grep -cE "$pattern" "$file" 2>/dev/null || true
}

# Squeeze a useful one-liner from each tool's captured output for the table.
#
# Each case branch must always exit 0 — `set -e` in the caller would
# otherwise blow up when a branch's last command (e.g. `[ N -gt 0 ] && …`)
# short-circuits to false. We use `if … fi` (which returns 0 if the
# condition is false and there's no else) and never end on a bare `&&`.
metric_for() {
    local id="$1"
    local file="${RUNNER_TEMP:-/tmp}/$2"
    [ -f "$file" ] || return 0
    case "$id" in
        lint)
            local issues
            issues=$(count_matches '^[^[:space:]][^:]*\.go:[0-9]+:[0-9]+:' "$file")
            if [ "${issues:-0}" -gt 0 ]; then echo "${issues} issue(s)"; fi
            ;;
        vet|build)
            # Go compiler / vet output: file.go:line:col: message
            local errs
            errs=$(count_matches '\.go:[0-9]+:[0-9]+:' "$file")
            if [ "${errs:-0}" -gt 0 ]; then echo "${errs} compile error(s)"; fi
            ;;
        test)
            local fails panics
            fails=$(count_matches '^--- FAIL:' "$file")
            panics=$(count_matches '^panic:' "$file")
            local parts=()
            if [ "${fails:-0}" -gt 0 ]; then parts+=("${fails} fail(s)"); fi
            if [ "${panics:-0}" -gt 0 ]; then parts+=("${panics} panic(s)"); fi
            if [ "${#parts[@]}" -gt 0 ]; then
                local IFS=', '
                echo "${parts[*]}"
            fi
            ;;
        smoke)
            local ok fail
            ok=$(count_matches '^OK ' "$file")
            fail=$(count_matches '^FAIL ' "$file")
            if [ "${fail:-0}" -gt 0 ]; then
                echo "${fail} fail(s), ${ok} ok"
            elif [ "${ok:-0}" -gt 0 ]; then
                echo "${ok} ok"
            fi
            ;;
        eslint)
            local line
            line=$(grep -E '[0-9]+ problems? \([0-9]+ errors' "$file" 2>/dev/null | tail -1 || true)
            if [ -n "$line" ]; then echo "$line"; fi
            ;;
        typecheck)
            local errs
            errs=$(count_matches '\.(ts|vue|tsx|mts|cts)\([0-9]+,[0-9]+\):' "$file")
            if [ "${errs:-0}" -gt 0 ]; then echo "${errs} type error(s)"; fi
            ;;
    esac
    return 0
}

# extract_findings prints a short bullet list of the most relevant lines for
# a given failing check — the kind of "here's what actually broke" surface
# that saves reading the whole captured log. Empty output when nothing
# notable is found (the <details> block below still shows the full capture).
extract_findings() {
    local id="$1"
    local file="${RUNNER_TEMP:-/tmp}/$2"
    [ -f "$file" ] || return 0
    case "$id" in
        build|vet)
            grep -E '\.go:[0-9]+:[0-9]+:' "$file" 2>/dev/null \
                | head -5 \
                | sed -e 's/^/- `/' -e 's/$/`/' || true
            ;;
        lint)
            grep -E '^[^[:space:]][^:]*\.go:[0-9]+:[0-9]+:' "$file" 2>/dev/null \
                | head -5 \
                | sed -e 's/^/- `/' -e 's/$/`/' || true
            ;;
        test)
            local names
            names=$(grep -E '^--- FAIL:' "$file" 2>/dev/null \
                | sed -e 's/^--- FAIL: //' -e 's/ (.*//' \
                | sort -u || true)
            if [ -n "$names" ]; then
                echo "$names" | head -5 | sed -e 's/^/- упал тест: `/' -e 's/$/`/'
            fi
            # Surface panic lines separately — they're qualitatively different
            # from a regular FAIL (often points at a nil deref or setup bug).
            grep -E '^panic:' "$file" 2>/dev/null \
                | head -2 \
                | sed -e 's/^/- `/' -e 's/$/`/' || true
            ;;
        smoke)
            grep -E '^FAIL ' "$file" 2>/dev/null | head -8 | sed 's/^/- /' || true
            ;;
        eslint)
            # Stylish format: file path on its own line, then "  L:C  error  msg  rule".
            # Collapse to one line per issue.
            awk '
                /^[^[:space:]].+\.[tjvm]/ { file=$0; next }
                /^[[:space:]]+[0-9]+:[0-9]+/ {
                    sub(/^[[:space:]]+/, "");
                    printf("- `%s` %s\n", file, $0);
                }
            ' "$file" 2>/dev/null | head -5 || true
            ;;
        typecheck)
            grep -E '\.(ts|vue|tsx|mts|cts)\([0-9]+,[0-9]+\):' "$file" 2>/dev/null \
                | head -5 \
                | sed -e 's/^/- `/' -e 's/$/`/' || true
            ;;
        oapi_v1|oapi_v2|sqlc|gofmt)
            # Drift checks already produce a short, human-readable summary;
            # the <details> block carries the full thing.
            :
            ;;
    esac
}

# ── Check definitions ─────────────────────────────────────────────────────
case "$target" in
    backend)
        title="Backend"
        checks=(
            "gofmt|R_GOFMT|gofmt.out|gofmt"
            "go vet|R_VET|vet.out|vet"
            "go build|R_BUILD|build.out|build"
            "go test|R_TEST|test.out|test"
            "golangci-lint|R_LINT|lint.out|lint"
            "OpenAPI v1 codegen drift|R_OAPI_V1|oapi_v1.out|oapi_v1"
            "OpenAPI v2 codegen drift|R_OAPI_V2|oapi_v2.out|oapi_v2"
            "sqlc codegen drift|R_SQLC|sqlc.out|sqlc"
            "smoke (curl happy-path)|R_SMOKE|smoke.out|smoke"
            "Docker image build|R_DOCKER||docker"
        )
        ;;
    frontend)
        title="Frontend"
        checks=(
            "oxlint|R_OXLINT|oxlint.out|oxlint"
            "eslint|R_ESLINT|eslint.out|eslint"
            "type-check|R_TYPECHECK|typecheck.out|typecheck"
            "build|R_BUILD|build.out|build"
            "Docker image build|R_DOCKER||docker"
        )
        ;;
    *)
        echo "unknown target: $target" >&2
        exit 2
        ;;
esac

# ── Aggregate ─────────────────────────────────────────────────────────────
overall="success"
failed_count=0
failed_ids=()
for c in "${checks[@]}"; do
    IFS='|' read -r _ var _ id <<< "$c"
    val="${!var:-skipped}"
    if [ "$val" = "failure" ]; then
        overall="failure"
        failed_count=$((failed_count + 1))
        failed_ids+=("$id")
    fi
done

if [ "$overall" = "success" ]; then
    opener_set=opener_pass; closer_set=closer_pass
elif [ "$failed_count" -eq 1 ]; then
    opener_set=opener_one;  closer_set=closer_one
else
    opener_set=opener_many; closer_set=closer_many
fi

# ── Render ────────────────────────────────────────────────────────────────
{
    echo "## CI — $title"
    echo
    echo "> $(pick_one "$opener_set")"
    echo

    total_count=${#checks[@]}
    if [ "$overall" = "success" ]; then
        echo "**Итог.** Все $total_count $(noun_check "$total_count") прошли — Аврора довольна."
    else
        # Capitalise the leading verb so it reads as a clean sentence after
        # "Итог." — bash ${var^} would be cleanest but isn't available on
        # every shell; sed is portable.
        verb=$(verb_fell "$failed_count" | sed 's/^./\U&/')
        echo "**Итог.** $verb $failed_count $(noun_check "$failed_count") из $total_count. Детали — ниже, читайте внимательно."
    fi
    echo

    trigger_label="$(case "${TRIGGER:-}" in
        pull_request)      echo "pull_request" ;;
        workflow_dispatch) echo "ручной перезапуск" ;;
        *)                 echo "${TRIGGER:-неизвестный}" ;;
    esac)"
    {
        echo -n "Триггер: \`$trigger_label\`"
        echo -n " · Коммит: [\`${GITHUB_SHA:0:7}\`](${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/commit/${GITHUB_SHA})"
        echo -n " · [Run #${GITHUB_RUN_NUMBER:-?}](${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID})"
        if [ -n "${JOB_URL:-}" ]; then
            echo -n " · [Job](${JOB_URL})"
        fi
        echo
    }
    echo

    # ── Quick-fix block ───────────────────────────────────────────────────
    if [ "$overall" = "failure" ]; then
        fixes=()
        for id in "${failed_ids[@]}"; do
            cmd="$(fix_command "$id")"
            [ -n "$cmd" ] && fixes+=("$cmd")
        done
        if [ "${#fixes[@]}" -gt 0 ]; then
            echo "<details open><summary>Быстрая починка — скопируйте и запустите</summary>"
            echo
            echo '```sh'
            for c in "${fixes[@]}"; do echo "$c"; done
            echo '```'
            echo
            echo "</details>"
            echo
        fi
    fi

    # ── Results table ─────────────────────────────────────────────────────
    echo "| | Проверка | Статус | Метрика |"
    echo "| --- | --- | --- | --- |"
    for c in "${checks[@]}"; do
        IFS='|' read -r label var file id <<< "$c"
        val="${!var:-skipped}"
        metric=""
        if [ -n "$file" ]; then
            metric="$(metric_for "$id" "$file")"
        fi
        echo "| $(dot_for "$val") | $label | $(label_for "$val") | ${metric:-—} |"
    done
    echo

    # ── Per-failure detail ────────────────────────────────────────────────
    any_failed=false
    for c in "${checks[@]}"; do
        IFS='|' read -r label var file id <<< "$c"
        val="${!var:-skipped}"
        [ "$val" = "failure" ] || continue
        any_failed=true
        echo "### $label"
        echo
        hint="$(hint_for "$id")"
        [ -n "$hint" ] && { echo "$hint"; echo; }
        findings="$(extract_findings "$id" "$file")"
        if [ -n "$findings" ]; then
            echo "**Что бросилось в глаза:**"
            echo
            echo "$findings"
            echo
        fi
        if [ -n "$file" ] && [ -f "${RUNNER_TEMP:-/tmp}/$file" ]; then
            cleaned="${RUNNER_TEMP:-/tmp}/${file}.clean"
            sed 's/\x1b\[[0-9;]*m//g' "${RUNNER_TEMP:-/tmp}/$file" \
                | head -c 12288 \
                | head -n 200 > "$cleaned"
            if [ -s "$cleaned" ]; then
                echo "<details><summary>что именно сказал инструмент</summary>"
                echo
                echo '```'
                cat "$cleaned"
                echo
                echo '```'
                echo
                echo "</details>"
                echo
            fi
        fi
    done

    if [ "$any_failed" = false ]; then
        echo "_Больше ничего не скажу — всё прошло._"
        echo
    fi

    echo "---"
    echo "_$(pick_one "$closer_set")_"
    echo

    # The help footer used to live in a <sub> block, but GitHub doesn't
    # process markdown inside inline HTML tags — so backtick-wrapped
    # commands rendered as literal text. Switching to a blockquote keeps
    # markdown rendering while still visually demoting the help section
    # below the actual results.
    echo "> **Что можно написать в комментарии:**"
    echo ">"
    echo "> - \`/ci\` — перезапустить проверки (доступно всем участникам)."
    echo "> - \`/ci backend\` или \`/ci frontend\` — сузить scope; добавьте \`skip-docker\`, чтобы пропустить сборку образов."
    echo "> - \`/label feature bugfix wip …\` — повесить метки на PR (автор PR, участники с правом записи или те, чьи коммиты уже в master)."
    echo "> - Те же команды через mention: \`@aurora <команда>\`."
    echo
    # The signature has no markdown to render, so a plain <sub> is fine
    # here — it just sets the size and stays inline.
    echo "<sub>С уважением, Аврора. Этот комментарий обновляется на каждый пуш.</sub>"
} > "$report"

cat "$report" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
