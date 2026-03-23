package auth

import (
	"context"
	"net/http"
)

// ProviderRedirect redirects to the OAuth provider.
func ProviderRedirect(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusFound)
}

// MeResult is the response for getting the current user.
type MeResult struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Me returns the current authenticated user.
func Me(ctx context.Context) (*MeResult, error) {
	return &MeResult{ID: "1", Name: "user", Email: "user@example.com"}, nil
}
