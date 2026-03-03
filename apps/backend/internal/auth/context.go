package auth

import "context"

type claimsContextKey struct{}
type deniedAuditContextKey struct{}

func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(*Claims)
	return claims, ok
}

func WithDeniedAuditRecorded(ctx context.Context) context.Context {
	return context.WithValue(ctx, deniedAuditContextKey{}, true)
}

func DeniedAuditRecorded(ctx context.Context) bool {
	recorded, ok := ctx.Value(deniedAuditContextKey{}).(bool)
	return ok && recorded
}
