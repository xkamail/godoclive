package arpc

import (
	"encoding/json"
	"net/http"
	"reflect"
)

// Manager wraps arpc handler functions into http.Handler.
type Manager struct{}

// New creates new arpc manager.
func New() *Manager {
	return &Manager{}
}

// Handler converts an arpc-style handler into an http.Handler.
func (m *Manager) Handler(f any) http.Handler {
	fv := reflect.ValueOf(f)
	ft := fv.Type()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		numIn := ft.NumIn()
		vIn := make([]reflect.Value, numIn)
		for i := 0; i < numIn; i++ {
			switch ft.In(i).String() {
			case "context.Context":
				vIn[i] = reflect.ValueOf(r.Context())
			default:
				infType := ft.In(i)
				if infType.Kind() == reflect.Ptr {
					infType = infType.Elem()
				}
				rfReq := reflect.New(infType)
				json.NewDecoder(r.Body).Decode(rfReq.Interface())
				vIn[i] = rfReq
			}
		}
		vOut := fv.Call(vIn)
		var res any
		if ft.NumOut() > 1 {
			res = vOut[0].Interface()
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": res})
	})
}

// MiddlewareContext provides request/response access in middleware.
type MiddlewareContext struct {
	r *http.Request
}

// Request returns the HTTP request.
func (ctx *MiddlewareContext) Request() *http.Request { return ctx.r }

// Middleware type for arpc.
type Middleware func(r *MiddlewareContext) error

// Middleware returns an HTTP middleware from an arpc middleware function.
func (m *Manager) Middleware(f Middleware) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		})
	}
}
