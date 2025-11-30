package handler

import "gorm.io/gorm"

// PaginationMeta defines the structure for pagination metadata.
type PaginationMeta struct {
	TotalItems  int64 `json:"total_items"`
	TotalPages  int   `json:"total_pages"`
	CurrentPage int   `json:"current_page"`
	PageSize    int   `json:"page_size"`
}

// PaginatedResponse defines the structure for a paginated list of any type.
type PaginatedResponse[T any] struct {
	Data []T            `json:"data"`
	Meta PaginationMeta `json:"meta"`
}

// NewPaginatedResponse creates a new PaginatedResponse.
func NewPaginatedResponse[T any](data []T, totalItems int64, page, limit int) PaginatedResponse[T] {
	if limit <= 0 {
		limit = 1
	}
	return PaginatedResponse[T]{
		Data: data,
		Meta: PaginationMeta{
			TotalItems:  totalItems,
			TotalPages:  (int(totalItems) + limit - 1) / limit,
			CurrentPage: page,
			PageSize:    limit,
		},
	}
}

// Paginate executes a paginated query and returns the results.
func Paginate[T any](db *gorm.DB, page, limit int) (*PaginatedResponse[T], error) {
	var totalItems int64
	if err := db.Model(new(T)).Count(&totalItems).Error; err != nil {
		return nil, err
	}

	var results []T
	offset := (page - 1) * limit
	if err := db.Offset(offset).Limit(limit).Find(&results).Error; err != nil {
		return nil, err
	}

	response := NewPaginatedResponse(results, totalItems, page, limit)
	return &response, nil
}
