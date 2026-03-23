package main

import (
	"encoding/json"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	Mount(mux)
	http.ListenAndServe(":8080", mux)
}

// Mount registers routes on an externally-provided ServeMux.
func Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /auth/{provider}", ProviderRedirect)
	mux.HandleFunc("GET /auth/{provider}/callback", ProviderCallback)
	mux.Handle("POST /site.list", protect(http.HandlerFunc(ListSites)))
	mux.Handle("POST /site.create", protect(http.HandlerFunc(CreateSite)))
}

func protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// ProviderRedirect redirects to the OAuth provider.
func ProviderRedirect(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	_ = provider
	w.WriteHeader(http.StatusFound)
}

// ProviderCallback handles the OAuth callback.
func ProviderCallback(w http.ResponseWriter, r *http.Request) {
	provider := r.PathValue("provider")
	_ = provider
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "authenticated"})
}

// SiteResponse is a site in the response.
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

// CreateSiteRequest is the request body for creating a site.
type CreateSiteRequest struct {
	Name string `json:"name"`
}

// CreateSite creates a new site.
func CreateSite(w http.ResponseWriter, r *http.Request) {
	var req CreateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SiteResponse{ID: "new", Name: req.Name})
}
