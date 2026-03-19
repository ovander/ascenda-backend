// Package dto defines Data Transfer Objects that decouple internal model
// representations from the JSON API contract. Handlers convert models to DTOs
// before serialisation, ensuring that internal schema changes (new GORM tags,
// renamed fields, added audit columns, etc.) never accidentally break the API.
package dto

import "time"

// PagedResponse wraps a paginated list of items with metadata.
type PagedResponse[T any] struct {
	Data  []T   `json:"data"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
}

// NewPagedResponse creates a PagedResponse with the given items and pagination info.
func NewPagedResponse[T any](data []T, total int64, page, limit int) PagedResponse[T] {
	if data == nil {
		data = []T{}
	}
	return PagedResponse[T]{
		Data:  data,
		Total: total,
		Page:  page,
		Limit: limit,
	}
}

// Timestamps embeds common time fields that most DTOs share.
type Timestamps struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
