package cart

import "context"

// AddParams is the request for adding an item to the cart.
type AddParams struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

// AddResult is the response for adding an item.
type AddResult struct {
	OK bool `json:"ok"`
}

// Add adds an item to the cart.
func Add(ctx context.Context, p *AddParams) (*AddResult, error) {
	return &AddResult{OK: true}, nil
}

// MeResult is the response for getting the current cart.
type MeResult struct {
	Items []string `json:"items"`
}

// Me returns the current cart.
func Me(ctx context.Context) (*MeResult, error) {
	return &MeResult{Items: []string{}}, nil
}
