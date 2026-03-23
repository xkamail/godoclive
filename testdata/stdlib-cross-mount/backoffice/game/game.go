package game

import "context"

// ListParams is the request for listing games.
type ListParams struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// ListResult is the response for listing games.
type ListResult struct {
	Items []GameItem `json:"items"`
	Total int        `json:"total"`
}

// GameItem is a single game.
type GameItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// List returns games.
func List(ctx context.Context, p *ListParams) (*ListResult, error) {
	return &ListResult{Total: 0}, nil
}

// CreateParams is the request for creating a game.
type CreateParams struct {
	Name string `json:"name"`
}

// CreateResult is the response for creating a game.
type CreateResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Create creates a new game.
func Create(ctx context.Context, p *CreateParams) (*CreateResult, error) {
	return &CreateResult{ID: "1", Name: p.Name}, nil
}
