package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// CreateUserRequest is the request body for creating a new user.
type CreateUserRequest struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required"`
	Age   int    `json:"age,omitempty"`
}

// UserResponse is the response body for user endpoints.
type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age,omitempty"`
}

// ErrorResponse is returned for all error cases.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

func main() {
	mux := http.NewServeMux()

	// Go 1.22+ enhanced patterns with method prefix.
	mux.HandleFunc("GET /users", ListUsers)
	mux.HandleFunc("POST /users", CreateUser)
	mux.HandleFunc("GET /users/{id}", GetUser)
	mux.HandleFunc("DELETE /users/{id}", DeleteUser)

	// Pattern without method prefix — matches any method.
	mux.HandleFunc("/health", HealthCheck)

	// http.Handler interface — struct with ServeHTTP.
	mux.Handle("GET /products/{id}", &ProductHandler{})

	http.ListenAndServe(":8080", mux)
}

// ListUsers returns a paginated list of users.
func ListUsers(w http.ResponseWriter, r *http.Request) {
	page := r.URL.Query().Get("page")
	if page == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error: "page parameter is required",
			Code:  http.StatusBadRequest,
		})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err == nil {
			limit = parsed
		}
	}

	_ = limit

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]UserResponse{
		{ID: "1", Name: "Alice", Email: "alice@example.com", Age: 30},
	})
}

// GetUser returns a single user by ID.
func GetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error: "missing user id",
			Code:  http.StatusBadRequest,
		})
		return
	}

	user := UserResponse{
		ID:    id,
		Name:  "Alice",
		Email: "alice@example.com",
		Age:   30,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

// CreateUser creates a new user from the JSON request body.
func CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "invalid request body",
			Code:    http.StatusBadRequest,
			Details: err.Error(),
		})
		return
	}

	user := UserResponse{
		ID:    "new-user-id",
		Name:  req.Name,
		Email: req.Email,
		Age:   req.Age,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// DeleteUser deletes a user by ID.
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error: "missing user id",
			Code:  http.StatusBadRequest,
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HealthCheck returns a simple health status.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ProductHandler demonstrates the http.Handler interface pattern.
type ProductHandler struct{}

// ServeHTTP handles GET /products/{id}.
func (h *ProductHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_ = id

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"id":   id,
		"name": "Widget",
	})
}
