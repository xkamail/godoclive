package main

import (
	"net/http"

	"github.com/xkamail/godoclive/testdata/stdlib-cross-mount/arpc"
	"github.com/xkamail/godoclive/testdata/stdlib-cross-mount/backoffice"
	"github.com/xkamail/godoclive/testdata/stdlib-cross-mount/httpmux"
)

func main() {
	mux := httpmux.New()
	am := arpc.New()

	// Health check at root
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Mount backoffice with /backoffice prefix
	backoffice.Mount(mux.Group("/backoffice"), am)

	http.ListenAndServe(":8080", mux)
}
