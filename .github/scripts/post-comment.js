// Post (or update in place) a sticky PR comment.
//
// Uses a hidden HTML marker so we never spam the PR with duplicates: on each
// run we look for an existing comment authored by the bot containing
// <!-- ci-marker:<marker> --> and edit it; if none exists, we open a new one.
//
// Invoked from actions/github-script@v9 — the caller must pass every input
// explicitly. We intentionally don't read context.repo or
// context.payload.pull_request because the @actions/github Context exposes
// `repo` as a getter on the class prototype, and spreading a Context to
// fake a different PR loses that getter (and you get "Cannot read
// properties of undefined (reading 'owner')" the next call).
//
// Required: { github, core, owner, repo, issueNumber, body, marker }
module.exports = async ({ github, core, owner, repo, issueNumber, body, marker }) => {
  if (!owner || !repo || !issueNumber || !body || !marker) {
    core.setFailed(
      `post-comment: missing required input (owner=${owner}, repo=${repo}, issueNumber=${issueNumber}, marker=${marker}, body.length=${body && body.length})`,
    )
    return
  }

  const sentinel = `<!-- ci-marker:${marker} -->`
  const fullBody = `${sentinel}\n${body}`

  // Paginate manually — the bot is rarely the source of many comments, but
  // we don't want to miss it because it's on page 2.
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner,
    repo,
    issue_number: issueNumber,
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
    core.info(`updated comment ${existing.id} on ${owner}/${repo}#${issueNumber}`)
  } else {
    const created = await github.rest.issues.createComment({
      owner,
      repo,
      issue_number: issueNumber,
      body: fullBody,
    })
    core.info(`created comment ${created.data.id} on ${owner}/${repo}#${issueNumber}`)
  }
}
