#!/usr/bin/env bash
# Render a Markdown CI report into:
#   - $GITHUB_STEP_SUMMARY  (the per-run summary tab on GitHub Actions)
#   - $RUNNER_TEMP/report.md (read back by post-comment.js for the PR comment)
#
# Voice belongs to our perpetually exasperated DevOps engineer. Decorative
# emojis are out — status circles in the table are functional (status
# indicators), so those stay.

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

# Deterministic-per-run pick from a bash array. Seed from run id + target +
# array name so the run summary and the PR comment show the same line.
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

# Russian plurals — проверка / проверки / проверок.
noun_check() {
    local n=$1; local lt=$((n % 100)); local lo=$((n % 10))
    if [ "$lt" -ge 11 ] && [ "$lt" -le 14 ]; then echo "проверок"
    elif [ "$lo" -eq 1 ]; then echo "проверка"
    elif [ "$lo" -ge 2 ] && [ "$lo" -le 4 ]; then echo "проверки"
    else echo "проверок"
    fi
}
# Russian verb agreement for past-tense "упасть".
verb_fell() {
    local n=$1; local lt=$((n % 100)); local lo=$((n % 10))
    if [ "$lt" -ge 11 ] && [ "$lt" -le 14 ]; then echo "упало"
    elif [ "$lo" -eq 1 ]; then echo "упала"
    elif [ "$lo" -ge 2 ] && [ "$lo" -le 4 ]; then echo "упали"
    else echo "упало"
    fi
}

# ── Openers ────────────────────────────────────────────────────────────────
opener_pass=(
    "Так. Сижу, кофе допиваю. Всё зелёное — спасибо, конечно, но я не привыкла."
    "Чисто. И я даже не нашла к чему придраться, что само по себе подозрительно."
    "Окей, прошло. Может я плохо проверяю, может ты случайно сделал хорошо. Время покажет."
    "Прошло с первого раза. Запиши себе в дневник, я тоже запишу."
    "Ну надо же. Ни одной красной строки. Кто-то постарался, и я надеюсь это ты."
    "Зелёный свет везде. Иди мёрджи, пока я не передумала."
    "Всё ок. Я даже расстроилась немножко — мне было что сказать. В следующий раз."
)
opener_one=(
    "Одна красная клетка. Знаешь что — почини её и не отвлекай меня по мелочам."
    "Одна проверка свалилась. Я уже даже не возмущаюсь, просто смотри и фикси."
    "Только одно сломано. Это почти достижение. Почти."
    "Так. Одна штука не сошлась. Думаю, ты сам всё уже видишь по списку ниже."
)
opener_many=(
    "Ну так. Садись, разбираем по пунктам. Я никуда не тороплюсь, у меня кофе."
    "Слушай, я не для того тут сижу, чтобы ловить за тебя такие очевидные вещи."
    "У нас с тобой системная проблема — третий PR подряд один и тот же список."
    "Я не злюсь. Я разочарована. Это страшнее, поверь."
    "Окей. Дыши, я тоже дышу. Всё это чинится, просто надо сесть и сделать."
    "Только не говори мне, что у тебя локально всё работало. Я этого больше не выдержу."
    "Так, без паники. Мы это уже видели — ну значит давай ещё раз."
)
closer_pass=(
    "Я пошла дальше нервничать в другие репы. Береги себя."
    "Если что — я тут. Но не сильно надейся, что часто."
    "Удачи на ревью. Имей в виду, люди иногда злее меня."
)
closer_one=(
    "Я подожду. Только не молча — напиши хоть что-нибудь в PR, когда починишь."
    "Поправь и пингани меня пушем. Я перепроверю — мне несложно, мне просто грустно."
    "Минута тебе. Максимум две."
)
closer_many=(
    "Пойду налью себе ещё чашку, ты пока разбирайся. Жду новый пуш."
    "Я никуда. Чини и пуш — посмотрим следующий заход."
    "Может в этот раз ты сначала локально прогонишь, а уже потом PR откроешь? Я просто спрашиваю."
    "Ладно, не буду давить. Но я записала."
)

# ── Hints — concrete fix commands in her voice ────────────────────────────
hint_for() {
    case "$1" in
        gofmt)
            echo "Это команда из трёх символов: \`gofmt -w .\` в \`backend/\`. Закоммить — и забудем."
            ;;
        vet)
            echo "\`go vet ./...\` падает на проде так же, как и тут. Не игнорируй."
            ;;
        build)
            echo "Проект не собирается. \`go build ./...\` локально, и не возвращайся пока не соберётся."
            ;;
        test)
            echo "Тесты упали. \`go test -race ./...\` дома, прежде чем приходить ко мне."
            ;;
        lint)
            echo "Линтер ругается. Inline-аннотации я подвесила прямо на diff PR — открой Files changed, увидишь красное на конкретных строках."
            ;;
        oapi_v1)
            echo "Спека v1 поменялась — кодоген не пересобран. \`cd backend && oapi-codegen -config oapi-codegen.yml api/openapi.yaml\`, потом \`git add internal/transport/http/v1/api.gen.go\`. Вручную там ничего не правь."
            ;;
        oapi_v2)
            echo "Спека v2 поменялась — кодоген не пересобран. \`cd backend && oapi-codegen -config oapi-codegen-v2.yml api/openapi-v2.yaml\`, дальше \`git add internal/transport/http/v2/api.gen.go\`."
            ;;
        sqlc)
            echo "SQL поменялся — \`storage/\` стало неактуальное. \`cd backend && sqlc generate -f internal/db/sqlc/sqlc.yaml\`, дальше сама знаешь."
            ;;
        docker)
            echo "Образ не собрался. Обычно это значит, что наверху уже что-то покраснело — почини то, и Docker подтянется. Если только docker — открывай логи джобы, там подробно."
            ;;
        oxlint)
            echo "Быстрый линтер ругается. \`bun run lint:oxlint\` локально, посмотри что говорит."
            ;;
        eslint)
            echo "ESLint. \`bun run lint:eslint --fix\` сначала — он половину сам поправит. Остальное руками."
            ;;
        typecheck)
            echo "Типы не сходятся. Не дави \`any\`, разбирайся: \`bun run type-check\`."
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

# Squeeze a useful one-liner from each tool's captured output for the table.
metric_for() {
    local id="$1"
    local file="${RUNNER_TEMP:-/tmp}/$2"
    [ -f "$file" ] || return 0
    case "$id" in
        lint)
            # golangci-lint prints "file.go:L:C: message (linter)" per issue.
            local issues
            issues=$(grep -cE '^[^[:space:]][^:]*\.go:[0-9]+:[0-9]+:' "$file" 2>/dev/null || echo 0)
            [ "$issues" -gt 0 ] && echo "$issues issue(s)"
            ;;
        test)
            # go test prints --- FAIL: TestX per failure.
            local fails
            fails=$(grep -cE '^--- FAIL:' "$file" 2>/dev/null || echo 0)
            [ "$fails" -gt 0 ] && echo "$fails fail(s)"
            ;;
        eslint)
            # eslint stylish prints "X problems (Y errors, Z warnings)".
            local line
            line=$(grep -E '[0-9]+ problems? \([0-9]+ errors' "$file" | tail -1 || true)
            [ -n "$line" ] && echo "$line"
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
        echo "**Итог:** все $total_count $(noun_check "$total_count") прошли. Это редкость, цени."
    else
        echo "**Итог:** $(verb_fell "$failed_count") $failed_count $(noun_check "$failed_count") из $total_count. Детали — ниже, читай внимательно."
    fi
    echo

    # Context line — trigger, commit, run link, optional job link.
    trigger_label="$(case "${TRIGGER:-}" in
        pull_request)     echo "pull_request" ;;
        workflow_dispatch) echo "manual rerun" ;;
        *)                echo "${TRIGGER:-unknown}" ;;
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

    # ── Quick-fix block (only when there are failures with a known command) ─
    if [ "$overall" = "failure" ]; then
        fixes=()
        for id in "${failed_ids[@]}"; do
            cmd="$(fix_command "$id")"
            [ -n "$cmd" ] && fixes+=("$cmd")
        done
        if [ "${#fixes[@]}" -gt 0 ]; then
            echo "<details open><summary>Быстрая починка — copy-paste и поехали</summary>"
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
        if [ -n "$file" ] && [ -f "${RUNNER_TEMP:-/tmp}/$file" ]; then
            cleaned="${RUNNER_TEMP:-/tmp}/${file}.clean"
            # Strip ANSI colour codes, then cap so we stay under GitHub's
            # 65 KB comment limit even with multiple failing steps.
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
    echo "<sub>"
    echo "Перезапуск из комментария: \`/ci\` (всё), \`/ci backend\` или \`/ci frontend\`, "
    echo "плюс \`skip-docker\`. Работает только для тех, у кого write-доступ к репе."
    echo "Этот комментарий обновляется на каждый пуш. С любовью (через слёзы), ваша CI."
    echo "</sub>"
} > "$report"

cat "$report" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
