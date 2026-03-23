package arpc

import (
	"encoding/json"
	"net/http"
	"reflect"
	"time"
)

// Manager wraps arpc handler functions into http.Handler.
type Manager struct {
	Validate bool
}

// New creates new arpc manager.
func New() *Manager {
	return &Manager{Validate: true}
}

// Handler converts an arpc-style handler into an http.Handler.
// The handler signature must be: func(context.Context, *Params) (*Result, error)
//
// Success response: {"ok": true, "result": <Result>}
// Error response:   {"ok": false, "error": {"code": "...", "message": "..."}}
func (m *Manager) Handler(f any) http.Handler {
	fv := reflect.ValueOf(f)
	ft := fv.Type()
	if ft.Kind() != reflect.Func {
		panic("arpc: f must be a function")
	}

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

		// Check error (last return)
		if errIdx := ft.NumOut() - 1; errIdx >= 0 {
			if vErr := vOut[errIdx]; !vErr.IsNil() {
				if err, ok := vErr.Interface().(error); ok && err != nil {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(ErrorResponse{
						OK:    false,
						Error: &ErrorDetail{Message: err.Error()},
					})
					return
				}
			}
		}

		// Encode success
		var res any
		if ft.NumOut() > 1 {
			res = vOut[0].Interface()
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(OKResponse{
			OK:     true,
			Result: res,
		})
	})
}

// OKResponse is the arpc success envelope.
type OKResponse struct {
	OK     bool `json:"ok"`
	Result any  `json:"result"`
}

// ErrorResponse is the arpc error envelope.
type ErrorResponse struct {
	OK    bool         `json:"ok"`
	Error *ErrorDetail `json:"error"`
}

// ErrorDetail contains error code and message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error is an arpc OKError (returns HTTP 200 with ok=false).
type Error struct {
	code string
	msg  string
}

func (e *Error) Error() string { return e.code + " " + e.msg }

// NewError creates a new Error with message.
func NewError(message string) error {
	return &Error{msg: message}
}

// NewErrorCode creates a new Error with code and message.
func NewErrorCode(code, message string) error {
	return &Error{code: code, msg: message}
}

// MiddlewareContext provides request/response access in middleware.
type MiddlewareContext struct {
	r *http.Request
	w http.ResponseWriter
}

// Request returns the HTTP request.
func (ctx *MiddlewareContext) Request() *http.Request { return ctx.r }

// ResponseWriter returns the HTTP response writer.
func (ctx *MiddlewareContext) ResponseWriter() http.ResponseWriter { return ctx.w }

// Deadline implements context.Context.
func (ctx *MiddlewareContext) Deadline() (time.Time, bool) { return ctx.r.Context().Deadline() }

// Done implements context.Context.
func (ctx *MiddlewareContext) Done() <-chan struct{} { return ctx.r.Context().Done() }

// Err implements context.Context.
func (ctx *MiddlewareContext) Err() error { return ctx.r.Context().Err() }

// Value implements context.Context.
func (ctx *MiddlewareContext) Value(key any) any { return ctx.r.Context().Value(key) }

// Middleware type.
type Middleware func(r *MiddlewareContext) error

// Middleware returns an HTTP middleware.
func (m *Manager) Middleware(f Middleware) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := MiddlewareContext{r, w}
			err := f(&ctx)
			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(ErrorResponse{
					OK:    false,
					Error: &ErrorDetail{Message: err.Error()},
				})
				return
			}
			h.ServeHTTP(ctx.w, ctx.r)
		})
	}
}
