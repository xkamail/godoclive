package marshaler

import "encoding/json"

// Paginate has only unexported fields but marshals to a fixed JSON object via a
// map literal — the shape must come from MarshalJSON, not the Go fields.
type Paginate struct {
	page  uint64
	limit uint64
	count int64
}

func (p Paginate) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"page":       p.page,
		"limit":      p.limit,
		"totalItems": p.count,
		"totalPages": (p.count + int64(p.limit) - 1) / int64(p.limit),
		"firstPage":  p.page == 1,
		"lastPage":   p.page*p.limit >= uint64(p.count),
	})
}

// Status is a stringer-style enum: a uint that marshals to a string via
// String(), with the JSON values living in the const line comments.
type Status uint

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

const (
	StatusNotPaid    Status = iota // not_paid
	StatusInProgress               // in_progress
	StatusSuccess                  // success
)

func (s Status) String() string {
	switch s {
	case StatusNotPaid:
		return "not_paid"
	case StatusInProgress:
		return "in_progress"
	case StatusSuccess:
		return "success"
	}
	return "unknown"
}

// Opaque implements MarshalJSON in a way we cannot statically analyze, so it
// must fall back to a generic object rather than exposing internal fields.
type Opaque struct {
	secret string
}

func (o Opaque) MarshalJSON() ([]byte, error) {
	return []byte(`{"computed":true}`), nil
}

// OrderResult uses a stringer enum as a field: the field must render as a
// string with the enum values.
type OrderResult struct {
	ID     string   `json:"id"`
	Status Status   `json:"status"`
	Page   Paginate `json:"page"`
}
