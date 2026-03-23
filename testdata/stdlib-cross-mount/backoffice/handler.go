package backoffice

import (
	"context"

	"github.com/xkamail/godoclive/testdata/stdlib-cross-mount/arpc"
	"github.com/xkamail/godoclive/testdata/stdlib-cross-mount/backoffice/game"
	"github.com/xkamail/godoclive/testdata/stdlib-cross-mount/httpmux"
)

func authMiddleware(ctx *arpc.MiddlewareContext) error {
	return nil
}

// Mount registers backoffice routes on the given mux.
func Mount(r *httpmux.Mux, am *arpc.Manager) {
	// Public route
	r.Handle("POST /auth.signIn", am.Handler(SignIn))

	// Protected group
	m := r.Group("", am.Middleware(authMiddleware))
	m.Handle("POST /game.list", am.Handler(game.List))
	m.Handle("POST /game.create", am.Handler(game.Create))
}

// SignInParams is the request for signing in.
type SignInParams struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SignInResult is the response for signing in.
type SignInResult struct {
	Token string `json:"token"`
}

// SignIn authenticates an admin user.
func SignIn(ctx context.Context, p *SignInParams) (*SignInResult, error) {
	return &SignInResult{Token: "test-token"}, nil
}
