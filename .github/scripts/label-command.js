// Apply a set of labels to a PR, creating missing well-known ones with our
// catalog's color + description on the fly. Returns a Markdown summary that
// the caller (pr-bot.yml) posts back as a PR comment.
//
// Required: { github, core, owner, repo, issueNumber, requested, actor }
//   requested: array of label names from the user's command
//   actor:     the comment author's login (used for "added by" attribution
//              and warning lines if some were skipped)
//
// Returns:    { summary: string, added: string[], skipped: { name, reason }[] }

const fs = require('fs')
const path = require('path')

module.exports = async ({ github, core, owner, repo, issueNumber, requested, actor }) => {
  // Load the catalog from disk relative to the repository root. The bot
  // workflow runs at the repo root, so this path is stable.
  const catalogPath = path.join(process.cwd(), '.github', 'labels.json')
  const catalog = JSON.parse(fs.readFileSync(catalogPath, 'utf8'))
  const knownByName = new Map(catalog.map((l) => [l.name.toLowerCase(), l]))

  // Normalise + dedupe requested names.
  const norm = (s) => s.toLowerCase().replace(/^@/, '').replace(/^\/+/, '')
  const wantedRaw = requested.map(norm).filter(Boolean)
  const wanted = [...new Set(wantedRaw)]

  const added = []
  const alreadyOn = []
  const skipped = [] // { name, reason }

  // Fetch current labels once so we can tell "already there" from "added".
  const existing = await github.paginate(github.rest.issues.listLabelsOnIssue, {
    owner,
    repo,
    issue_number: issueNumber,
    per_page: 100,
  })
  const existingSet = new Set(existing.map((l) => l.name.toLowerCase()))

  // The actual apply step uses addLabels, which is idempotent — labels
  // already present aren't duplicated. We still iterate one-by-one so we
  // can ensure the label exists in the repo first (auto-create from the
  // catalog when missing).
  for (const name of wanted) {
    const def = knownByName.get(name)
    if (!def) {
      skipped.push({ name, reason: 'я такого не знаю' })
      continue
    }
    // Ensure the label exists in the repo, creating it if not. We refresh
    // color/description every call so the canonical catalog stays the
    // source of truth.
    try {
      await github.rest.issues.getLabel({ owner, repo, name: def.name })
      // Re-sync color/description in case they drifted in the UI. Cheap.
      await github.rest.issues.updateLabel({
        owner,
        repo,
        name: def.name,
        new_name: def.name,
        color: def.color,
        description: def.description,
      })
    } catch (e) {
      if (e.status === 404) {
        await github.rest.issues.createLabel({
          owner,
          repo,
          name: def.name,
          color: def.color,
          description: def.description,
        })
      } else {
        skipped.push({ name, reason: `ошибка обращения: ${e.message}` })
        continue
      }
    }

    if (existingSet.has(def.name.toLowerCase())) {
      alreadyOn.push(def.name)
      continue
    }

    try {
      await github.rest.issues.addLabels({
        owner,
        repo,
        issue_number: issueNumber,
        labels: [def.name],
      })
      added.push(def.name)
    } catch (e) {
      skipped.push({ name: def.name, reason: `не получилось повесить: ${e.message}` })
    }
  }

  // ── Markdown summary in Aurora's voice ─────────────────────────────────
  const lines = []
  if (added.length || alreadyOn.length || skipped.length) {
    lines.push(`@${actor}, я посмотрела ваши метки.`)
    lines.push('')
  } else {
    lines.push(`@${actor}, я не нашла, что добавить — список меток пустой.`)
  }

  if (added.length) {
    lines.push('**Добавила:**')
    lines.push('')
    for (const n of added) {
      const def = knownByName.get(n.toLowerCase())
      lines.push(`- \`${n}\` — ${def.description}`)
    }
    lines.push('')
  }

  if (alreadyOn.length) {
    lines.push('**Уже стояли:**')
    lines.push('')
    for (const n of alreadyOn) {
      lines.push(`- \`${n}\``)
    }
    lines.push('')
  }

  if (skipped.length) {
    lines.push('**Не добавила:**')
    lines.push('')
    for (const s of skipped) {
      lines.push(`- \`${s.name}\` — ${s.reason}`)
    }
    lines.push('')
    const catalogList = catalog
      .map((l) => `\`${l.name}\``)
      .reduce((acc, cur, i, arr) => {
        const lineIdx = Math.floor(i / 6)
        acc[lineIdx] = acc[lineIdx] ? `${acc[lineIdx]}, ${cur}` : cur
        return acc
      }, [])
      .join('  \n')
    lines.push('Я знаю вот эти:')
    lines.push('')
    lines.push(catalogList)
    lines.push('')
  }

  lines.push('— Аврора')

  return {
    summary: lines.join('\n'),
    added,
    alreadyOn,
    skipped,
  }
}
