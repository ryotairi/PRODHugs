#!/usr/bin/env bash
# Render a Markdown CI report into:
#   - $GITHUB_STEP_SUMMARY  (the per-run summary tab on GitHub Actions)
#   - $RUNNER_TEMP/report.md (read back by post-comment.js for the PR comment)
#
# The voice belongs to our perpetually exasperated DevOps engineer. She is
# not happy about cleaning up after you again, but she has scripts and she
# will use them. Decorative emojis are out — status circles in the table
# are functional (status indicators), so those stay.

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

# Random-but-deterministic-per-run pick. $$ alone is constant within a run,
# RANDOM varies per call — we want the same answer every time this script
# runs in a given job (or you get whiplash between summary and comment), so
# seed from the run id + the target.
pick_one() {
    local arr_name="$1"
    eval "local arr=(\"\${${arr_name}[@]}\")"
    local len="${#arr[@]}"
    local seed="${GITHUB_RUN_ID:-0}${target}${arr_name}"
    # Crude string -> integer hash. Sufficient for picking from <20 items.
    local n=0 i
    for (( i=0; i<${#seed}; i++ )); do
        n=$(( (n * 31 + $(printf '%d' "'${seed:$i:1}")) & 0x7fffffff ))
    done
    echo "${arr[$(( n % len ))]}"
}

# Russian plural agreement for "проверка" — 1 проверка, 2 проверки, 5 проверок.
noun_check() {
    local n=$1
    local last_two=$((n % 100))
    local last_one=$((n % 10))
    if [ "$last_two" -ge 11 ] && [ "$last_two" -le 14 ]; then echo "проверок"
    elif [ "$last_one" -eq 1 ]; then echo "проверка"
    elif [ "$last_one" -ge 2 ] && [ "$last_one" -le 4 ]; then echo "проверки"
    else echo "проверок"
    fi
}

# Russian verb agreement for "упасть" past tense — 1 упала, 2-4 упали, 5+ упало.
verb_fell() {
    local n=$1
    local last_two=$((n % 100))
    local last_one=$((n % 10))
    if [ "$last_two" -ge 11 ] && [ "$last_two" -le 14 ]; then echo "упало"
    elif [ "$last_one" -eq 1 ]; then echo "упала"
    elif [ "$last_one" -ge 2 ] && [ "$last_one" -le 4 ]; then echo "упали"
    else echo "упало"
    fi
}

# ── Openers ────────────────────────────────────────────────────────────────
# All-green: ranges from grudging approval to mild suspicion. She doesn't
# want to admit it went well.
opener_pass=(
    "Так. Сижу, кофе допиваю. Всё зелёное — спасибо, конечно, но я не привыкла."
    "Чисто. И я даже не нашла к чему придраться, что само по себе подозрительно."
    "Окей, прошло. Может я плохо проверяю, может ты случайно сделал хорошо. Время покажет."
    "Прошло с первого раза. Запиши себе в дневник, я тоже запишу."
    "Ну надо же. Ни одной красной строки. Кто-то постарался, и я надеюсь это ты."
    "Зелёный свет везде. Иди мёрджи, пока я не передумала."
    "Всё ок. Я даже расстроилась немножко — мне было что сказать. В следующий раз."
)

# Single-failure: pointed but not cruel. One thing, fиксируй и приходи назад.
opener_one=(
    "Одна красная клетка. Знаешь что — почини её и не отвлекай меня по мелочам."
    "Одна проверка свалилась. Я уже даже не возмущаюсь, просто смотри и фикси."
    "Только одно сломано. Это почти достижение. Почти."
    "Так. Одна штука не сошлась. Думаю, ты сам всё уже видишь по списку ниже."
)

# Multi-failure: full sigh. Tonal range: tired, sarcastic, mama-bear strict.
opener_many=(
    "Ну так. Садись, разбираем по пунктам. Я никуда не тороплюсь, у меня кофе."
    "Слушай, я не для того тут сижу, чтобы ловить за тебя такие очевидные вещи."
    "У нас с тобой системная проблема — третий PR подряд один и тот же список."
    "Я не злюсь. Я разочарована. Это страшнее, поверь."
    "Окей. Дыши, я тоже дышу. Всё это чинится, просто надо сесть и сделать."
    "Только не говори мне, что у тебя локально всё работало. Я этого больше не выдержу."
    "Так, без паники. Мы это уже видели — ну значит давай ещё раз."
)

# Closer line — varies based on the result. Adds a small parting note so
# the bot doesn't feel like a stamping machine.
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

# ── Hints — concrete fix commands, framed in her voice ────────────────────
hint_for() {
    case "$1" in
        gofmt)
            echo "Это команда из трёх символов: \`gofmt -w .\` в \`backend/\`. Закоммить — и забудем."
            ;;
        vet)
            echo "\`go vet ./...\` падает на проде так же, как и тут. Это не вкусовщина — это реальная штука."
            ;;
        build)
            echo "Проект не собирается. Я понимаю, у тебя локально \"всё работало\" — но я вижу что нет. \`go build ./...\` тебе всё расскажет."
            ;;
        test)
            echo "Тесты упали. Я не знаю что именно — посмотри сама, и не забывай: \`go test -race ./...\` дома, прежде чем ко мне приходить."
            ;;
        lint)
            echo "Линтер ругается. Inline-аннотации я подвесила прямо на diff PR — открой Files changed, увидишь красное на конкретных строках. Полный список ниже."
            ;;
        oapi_v1)
            echo "Спека v1 поменялась — кодоген не пересобран. Лекарство: \`cd backend && oapi-codegen -config oapi-codegen.yml api/openapi.yaml\`, потом \`git add internal/transport/http/v1/api.gen.go\`. Я серьёзно, не вручную там ничего не правь."
            ;;
        oapi_v2)
            echo "Спека v2 поменялась — кодоген не пересобран. Лекарство: \`cd backend && oapi-codegen -config oapi-codegen-v2.yml api/openapi-v2.yaml\`, потом коммить \`internal/transport/http/v2/api.gen.go\`."
            ;;
        sqlc)
            echo "SQL поменялся — \`storage/\` стало неактуальное. \`cd backend && sqlc generate -f internal/db/sqlc/sqlc.yaml\`, дальше сама знаешь."
            ;;
        docker)
            echo "Образ не собрался. Обычно это значит, что наверху уже что-то покраснело — почини то, и Docker подтянется. Если только docker и больше ничего — открывай логи джобы, там подробно."
            ;;
        oxlint)
            echo "Быстрый линтер ругается. \`bun run lint:oxlint\` локально, посмотри что говорит — обычно лечится за минуту."
            ;;
        eslint)
            echo "ESLint. Сначала \`bun run lint:eslint --fix\` — он половину сам поправит. Что останется — то уж сама. Список ниже."
            ;;
        typecheck)
            echo "Типы не сходятся. Не игнорируй, не дави \`any\`, разбирайся: \`bun run type-check\`."
            ;;
        *) echo "" ;;
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
for c in "${checks[@]}"; do
    IFS='|' read -r _ var _ _ <<< "$c"
    val="${!var:-skipped}"
    if [ "$val" = "failure" ]; then
        overall="failure"
        failed_count=$((failed_count + 1))
    fi
done

if [ "$overall" = "success" ]; then
    opener_set=opener_pass
    closer_set=closer_pass
elif [ "$failed_count" -eq 1 ]; then
    opener_set=opener_one
    closer_set=closer_one
else
    opener_set=opener_many
    closer_set=closer_many
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
    echo "Коммит: \`${GITHUB_SHA:0:7}\` · [полный лог запуска](${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID})"
    echo
    echo "| | Проверка | Статус |"
    echo "| --- | --- | --- |"
    for c in "${checks[@]}"; do
        IFS='|' read -r label var _ _ <<< "$c"
        val="${!var:-skipped}"
        echo "| $(dot_for "$val") | $label | $(label_for "$val") |"
    done
    echo

    any_failed=false
    for c in "${checks[@]}"; do
        IFS='|' read -r label var file id <<< "$c"
        val="${!var:-skipped}"
        [ "$val" = "failure" ] || continue
        any_failed=true
        echo "### $label"
        echo
        hint="$(hint_for "$id")"
        if [ -n "$hint" ]; then
            echo "$hint"
            echo
        fi
        if [ -n "$file" ] && [ -f "${RUNNER_TEMP:-/tmp}/$file" ]; then
            # Strip ANSI colour codes that show up in lint/test output —
            # they render as garbage inside <pre>.
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
    echo "<sub>Этот комментарий обновляется на каждый пуш в PR. С любовью (через слёзы), ваша CI.</sub>"
} > "$report"

cat "$report" >> "${GITHUB_STEP_SUMMARY:-/dev/stdout}"
