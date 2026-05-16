// Package authctx defines the context keys used by the HTTP transport layer
// to propagate the authenticated user identity from auth middleware to
// handlers. It is its own package so handler packages and middleware can
// share the keys without forming an import cycle.
package authctx

type contextKey string

const (
	UserIDContextKey   contextKey = "user_id"
	UserRoleContextKey contextKey = "user_role"
)
