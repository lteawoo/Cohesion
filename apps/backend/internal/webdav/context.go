package webdav

import "context"

type contextKey string

const webDAVUsernameContextKey contextKey = "webdav.username"

func WithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, webDAVUsernameContextKey, username)
}

func UsernameFromContext(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(webDAVUsernameContextKey).(string)
	if !ok || username == "" {
		return "", false
	}
	return username, true
}
