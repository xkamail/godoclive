package main

import (
	"net/http"
	"strings"

	"github.com/xkamail/godoclive/testdata/stdlib-arpc/arpc"
	"github.com/xkamail/godoclive/testdata/stdlib-arpc/auth"
	"github.com/xkamail/godoclive/testdata/stdlib-arpc/cart"
	"github.com/xkamail/godoclive/testdata/stdlib-arpc/httpmux"
	"github.com/xkamail/godoclive/testdata/stdlib-arpc/site"
)

func main() {
	mux := httpmux.New()
	am := &arpc.Manager{}
	Mount(mux, am)
	http.ListenAndServe(":8080", mux)
}

var errUnauthorized = arpc.NewErrorCode("unauthorized", "unauthorized")

func authMiddleware(actx *arpc.MiddlewareContext) error {
	h := actx.Request().Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return errUnauthorized
	}
	return nil
}

// Mount registers all routes.
func Mount(mux *httpmux.Mux, am *arpc.Manager) {
	// Public HTTP handler
	mux.HandleFunc("GET /auth/{provider}", auth.ProviderRedirect)

	// Public arpc handlers
	mux.Handle("POST /site.list", am.Handler(site.List))
	mux.Handle("POST /site.create", am.Handler(site.Create))

	// Protected arpc routes (bearer token auth)
	a := mux.Group("", am.Middleware(authMiddleware))
	a.Handle("POST /auth.me", am.Handler(auth.Me))

	// Chained middleware in a bare block (rate limiting)
	{
		rl := rateLimitMiddleware()
		a.Middleware(rl).Handle("POST /cart.add", am.Handler(cart.Add))
		a.Middleware(rl).Handle("POST /cart.me", am.Handler(cart.Me))
	}
}

func rateLimitMiddleware() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return h
	}
}
