package game

import "context"

// ListItem is a single game in the list (DUPLICATE name with user.ListItem).
type ListItem struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Priority int    `json:"priority"`
	Hidden   bool   `json:"hidden"`
}

// ListResult is the response for listing games.
type ListResult struct {
	Items []ListItem `json:"items"`
	Total int        `json:"total"`
}

// List returns a paginated list of games.
func List(ctx context.Context, p *struct{}) (*ListResult, error) {
	return &ListResult{}, nil
}
