package user

import "context"

// ListItem is a single user in the list (DUPLICATE name with game.ListItem).
type ListItem struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// ListResult is the response for listing users.
type ListResult struct {
	Items []ListItem `json:"items"`
	Total int        `json:"total"`
}

// List returns a paginated list of users.
func List(ctx context.Context, p *struct{}) (*ListResult, error) {
	return &ListResult{}, nil
}
