// Post (or update in place) a sticky PR comment with the rendered CI report.
//
// Uses a hidden HTML marker so we never spam the PR with duplicates: on each
// run we look for an existing comment authored by github-actions[bot] that
// contains <!-- ci-marker:<marker> --> and edit it; if none exists we open
// a new one.
//
// Exported for use from actions/github-script:
//   await require('./.github/scripts/post-comment.js')({ github, context, core, body, marker })

module.exports = async ({ github, context, core, body, marker }) => {
  if (!context.payload.pull_request) {
    core.info('not a pull_request event; skipping comment')
    return
  }

  const owner = context.repo.owner
  const repo = context.repo.repo
  const issue_number = context.payload.pull_request.number
  const sentinel = `<!-- ci-marker:${marker} -->`
  const fullBody = `${sentinel}\n${body}`

  // Paginate manually — the bot is rarely the source of many comments, but
  // we don't want to miss it because it's on page 2.
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner,
    repo,
    issue_number,
    per_page: 100,
  })

  const existing = comments.find(
    (c) =>
      c.user &&
      (c.user.type === 'Bot' || c.user.login === 'github-actions[bot]') &&
      typeof c.body === 'string' &&
      c.body.includes(sentinel),
  )

  if (existing) {
    await github.rest.issues.updateComment({
      owner,
      repo,
      comment_id: existing.id,
      body: fullBody,
    })
    core.info(`updated comment ${existing.id}`)
  } else {
    const created = await github.rest.issues.createComment({
      owner,
      repo,
      issue_number,
      body: fullBody,
    })
    core.info(`created comment ${created.data.id}`)
  }
}
