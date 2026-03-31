package httpmux

import "net/http"

// Mux wraps http.ServeMux with middleware group support.
type Mux struct {
	*http.ServeMux
}

// New creates a new Mux.
func New() *Mux {
	return &Mux{ServeMux: http.NewServeMux()}
}

// Group creates a sub-group with the given prefix and middleware.
func (m *Mux) Group(prefix string, mw ...func(http.Handler) http.Handler) *Mux {
	return &Mux{ServeMux: m.ServeMux}
}
