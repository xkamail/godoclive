package main

import (
	"context"
	"net/http"

	"github.com/syst3mctl/godoclive/testdata/stdlib-arpc/arpc"
	"github.com/syst3mctl/godoclive/testdata/stdlib-arpc/auth"
	"github.com/syst3mctl/godoclive/testdata/stdlib-arpc/site"
)

func main() {
	mux := http.NewServeMux()
	am := &arpc.Manager{}
	Mount(mux, am)
	http.ListenAndServe(":8080", mux)
}

func authMiddleware(ctx context.Context) (context.Context, error) {
	return ctx, nil
}

func protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// Mount registers all routes.
func Mount(mux *http.ServeMux, am *arpc.Manager) {
	// Public HTTP handler
	mux.HandleFunc("GET /auth/{provider}", auth.ProviderRedirect)

	// arpc handlers
	mux.Handle("POST /site.list", am.Handler(site.List))
	mux.Handle("POST /site.create", am.Handler(site.Create))

	// Protected arpc handler
	mux.Handle("POST /auth.me", protect(am.Handler(auth.Me)))
}
