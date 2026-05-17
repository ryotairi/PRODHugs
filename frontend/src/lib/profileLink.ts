// Canonical URL builder for a user profile.
//
// Always prefer the @username form — it's the shareable, human-readable
// route — and only fall back to the UUID if the username is somehow missing
// (which shouldn't happen in any of our current payloads but keeps the
// helper safe for new data sources).
//
// /user/@:username is the canonical route in src/router/index.ts; the legacy
// /user/:id route still exists and router.replace()-redirects to the @
// form once the profile loads.
export function profileLink(username: string | null | undefined, fallbackId?: string | null): string {
  if (username) return `/user/@${username}`
  return `/user/${fallbackId ?? ''}`
}
