package httpapi

import (
	"context"
	"net/http"
	"pvmss/server/internal/auth"
	"time"
)

// defaultSessionCheckEvery bounds how long an open console or metrics stream
// outlives its session: logout, revocation or expiry closes it within one tick.
const defaultSessionCheckEvery = 30 * time.Second

// watchSession re-runs check every interval until ctx ends. The first failed
// check calls onEnd once and stops. Any error counts as ended (fail closed),
// as Resolve does for a request.
func watchSession(ctx context.Context, every time.Duration, check func(context.Context) error, onEnd func()) {
	if every <= 0 {
		every = defaultSessionCheckEvery
	}

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := check(ctx); err != nil {
				if ctx.Err() == nil {
					onEnd()
				}

				return
			}
		}
	}
}

// sessionChecker returns a check that the browser session behind r is still
// usable. The cookie is read once, at open time, like the identity.
func (h *Auth) sessionChecker(r *http.Request) func(context.Context) error {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return func(context.Context) error { return auth.ErrUnauthenticated }
	}

	raw := cookie.Value

	return func(ctx context.Context) error {
		return h.sessions.CheckSession(ctx, raw)
	}
}
