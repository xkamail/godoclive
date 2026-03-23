package site

import (
	"context"

	"github.com/syst3mctl/godoclive/testdata/stdlib-arpc/arpc"
)

// ListParams is the request for listing sites.
type ListParams struct {
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
	Query string `json:"query"`
}

// ListResult is the response for listing sites.
type ListResult struct {
	Items []SiteItem `json:"items"`
	Total int        `json:"total"`
}

// SiteItem is a single site in the list.
type SiteItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// List returns a paginated list of sites.
func List(ctx context.Context, p *ListParams) (*ListResult, error) {
	if p.Page < 1 {
		return nil, arpc.NewErrorCode("site/invalid-page", "page must be >= 1")
	}
	return &ListResult{
		Items: []SiteItem{{ID: "1", Name: "example"}},
		Total: 1,
	}, nil
}

// CreateParams is the request for creating a site.
type CreateParams struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// CreateResult is the response for creating a site.
type CreateResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// Create creates a new site.
func Create(ctx context.Context, p *CreateParams) (*CreateResult, error) {
	if p.Name == "" {
		return nil, arpc.NewError("name is required")
	}
	if p.Domain == "" {
		return nil, arpc.NewErrorCode("site/invalid-domain", "domain is required")
	}
	return &CreateResult{
		ID:     "new-id",
		Name:   p.Name,
		Domain: p.Domain,
	}, nil
}
