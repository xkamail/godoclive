package main

import (
	"encoding/json"
	"net/http"

	"github.com/xkamail/godoclive/testdata/stdlib-wrapper/httpmux"
)

func main() {
	mux := httpmux.New()
	Mount(mux)
	http.ListenAndServe(":8080", mux)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// Mount registers routes on a custom httpmux.Mux wrapper.
func Mount(mux *httpmux.Mux) {
	// Public routes
	mux.HandleFunc("GET /auth/{provider}", ProviderRedirect)
	mux.HandleFunc("GET /auth/callback", ProviderCallback)
	mux.HandleFunc("POST /ingest/{serial_number}", IngestHandler)

	// Protected group
	a := mux.Group("", authMiddleware)
	a.Handle("POST /auth.me", http.HandlerFunc(Me))
	a.Handle("POST /site.list", http.HandlerFunc(ListSites))
}

// ProviderRedirect redirects to the OAuth provider.
func ProviderRedirect(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusFound)
}

// ProviderCallback handles the OAuth callback.
func ProviderCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// IngestHandler receives device data.
func IngestHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusAccepted)
}

// MeResponse is the auth.me response.
type MeResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Me returns the current user.
func Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(MeResponse{ID: "1", Name: "user"})
}

// SiteResponse is a site.
type SiteResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListSites returns all sites.
func ListSites(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]SiteResponse{{ID: "1", Name: "example"}})
}
