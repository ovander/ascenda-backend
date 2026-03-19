package pagination

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		queryString   string
		expectedPage  int
		expectedPerPage int
		expectedOffset int
	}{
		{
			name:          "default values",
			queryString:   "",
			expectedPage:  1,
			expectedPerPage: DefaultPerPage,
			expectedOffset: 0,
		},
		{
			name:          "page 1 per_page 10",
			queryString:   "?page=1&per_page=10",
			expectedPage:  1,
			expectedPerPage: 10,
			expectedOffset: 0,
		},
		{
			name:          "page 2 per_page 20",
			queryString:   "?page=2&per_page=20",
			expectedPage:  2,
			expectedPerPage: 20,
			expectedOffset: 20,
		},
		{
			name:          "page 5 per_page 50",
			queryString:   "?page=5&per_page=50",
			expectedPage:  5,
			expectedPerPage: 50,
			expectedOffset: 200,
		},
		{
			name:          "page 0 defaults to 1",
			queryString:   "?page=0&per_page=10",
			expectedPage:  1,
			expectedPerPage: 10,
			expectedOffset: 0,
		},
		{
			name:          "negative page defaults to 1",
			queryString:   "?page=-5&per_page=10",
			expectedPage:  1,
			expectedPerPage: 10,
			expectedOffset: 0,
		},
		{
			name:          "per_page 0 defaults to DefaultPerPage",
			queryString:   "?page=1&per_page=0",
			expectedPage:  1,
			expectedPerPage: DefaultPerPage,
			expectedOffset: 0,
		},
		{
			name:          "per_page exceeds MaxPerPage",
			queryString:   "?page=1&per_page=150",
			expectedPage:  1,
			expectedPerPage: MaxPerPage,
			expectedOffset: 0,
		},
		{
			name:          "per_page negative defaults to DefaultPerPage",
			queryString:   "?page=1&per_page=-10",
			expectedPage:  1,
			expectedPerPage: DefaultPerPage,
			expectedOffset: 0,
		},
		{
			name:          "invalid page parameter",
			queryString:   "?page=abc&per_page=10",
			expectedPage:  1,
			expectedPerPage: 10,
			expectedOffset: 0,
		},
		{
			name:          "only page specified",
			queryString:   "?page=3",
			expectedPage:  3,
			expectedPerPage: DefaultPerPage,
			expectedOffset: 40,
		},
		{
			name:          "only per_page specified",
			queryString:   "?per_page=50",
			expectedPage:  1,
			expectedPerPage: 50,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://localhost"+tt.queryString, nil)
			result := Parse(req)

			assert.Equal(t, tt.expectedPage, result.Page)
			assert.Equal(t, tt.expectedPerPage, result.PerPage)
			assert.Equal(t, tt.expectedOffset, result.Offset)
		})
	}
}

func TestNewPagedResponse(t *testing.T) {
	tests := []struct {
		name        string
		data        any
		params      Params
		totalItems  int64
		expectedPages int
	}{
		{
			name:        "20 items, 20 per page",
			data:        []string{"a", "b"},
			params:      Params{Page: 1, PerPage: 20, Offset: 0},
			totalItems:  20,
			expectedPages: 1,
		},
		{
			name:        "50 items, 20 per page",
			data:        []string{},
			params:      Params{Page: 1, PerPage: 20, Offset: 0},
			totalItems:  50,
			expectedPages: 3,
		},
		{
			name:        "100 items, 25 per page",
			data:        []string{},
			params:      Params{Page: 1, PerPage: 25, Offset: 0},
			totalItems:  100,
			expectedPages: 4,
		},
		{
			name:        "10 items, 100 per page",
			data:        []string{},
			params:      Params{Page: 1, PerPage: 100, Offset: 0},
			totalItems:  10,
			expectedPages: 1,
		},
		{
			name:        "0 items",
			data:        []string{},
			params:      Params{Page: 1, PerPage: 20, Offset: 0},
			totalItems:  0,
			expectedPages: 0,
		},
		{
			name:        "51 items, 20 per page (3 pages total)",
			data:        []string{},
			params:      Params{Page: 2, PerPage: 20, Offset: 20},
			totalItems:  51,
			expectedPages: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := NewPagedResponse(tt.data, tt.params, tt.totalItems)

			assert.Equal(t, tt.params.Page, response.Page)
			assert.Equal(t, tt.params.PerPage, response.PerPage)
			assert.Equal(t, tt.totalItems, response.TotalItems)
			assert.Equal(t, tt.expectedPages, response.TotalPages)
			assert.Equal(t, tt.data, response.Data)
		})
	}
}

func TestOffsetCalculation(t *testing.T) {
	tests := []struct {
		name           string
		page           int
		perPage        int
		expectedOffset int
	}{
		{
			name:           "page 1",
			page:           1,
			perPage:        20,
			expectedOffset: 0,
		},
		{
			name:           "page 2",
			page:           2,
			perPage:        20,
			expectedOffset: 20,
		},
		{
			name:           "page 3 with 50 per page",
			page:           3,
			perPage:        50,
			expectedOffset: 100,
		},
		{
			name:           "page 10 with 100 per page",
			page:           10,
			perPage:        100,
			expectedOffset: 900,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := Params{Page: tt.page, PerPage: tt.perPage, Offset: (tt.page - 1) * tt.perPage}
			assert.Equal(t, tt.expectedOffset, params.Offset)
		})
	}
}

func TestMaxPerPageCap(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost?page=1&per_page=999", nil)
	result := Parse(req)

	assert.Equal(t, MaxPerPage, result.PerPage)
	assert.LessOrEqual(t, result.PerPage, 100)
}

func TestDefaultPerPage(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://localhost?page=1", nil)
	result := Parse(req)

	assert.Equal(t, DefaultPerPage, result.PerPage)
}
