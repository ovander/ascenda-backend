package pagination

import (
	"net/http"
	"strconv"
)

// Params holds parsed pagination parameters.
type Params struct {
	Page    int `json:"page"`
	PerPage int `json:"perPage"`
	Offset  int `json:"-"`
}

// DefaultPerPage is the default number of items per page.
const DefaultPerPage = 20

// MaxPerPage is the maximum number of items per page.
const MaxPerPage = 100

// Parse extracts pagination parameters from query string.
func Parse(r *http.Request) Params {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = DefaultPerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}

	return Params{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}
}

// PagedResponse wraps a list result with pagination metadata.
type PagedResponse struct {
	Data       any   `json:"data"`
	Page       int   `json:"page"`
	PerPage    int   `json:"perPage"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int   `json:"totalPages"`
}

// NewPagedResponse creates a paginated response.
func NewPagedResponse(data any, params Params, totalItems int64) PagedResponse {
	totalPages := int(totalItems) / params.PerPage
	if int(totalItems)%params.PerPage > 0 {
		totalPages++
	}
	return PagedResponse{
		Data:       data,
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalItems: totalItems,
		TotalPages: totalPages,
	}
}
