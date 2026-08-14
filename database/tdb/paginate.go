package tdb

import (
	"math"
)

// PaginateResult 分页查询结果。
type PaginateResult[T any] struct {
	Items       []T   `json:"items"`
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
	HasMore     bool  `json:"has_more"`
}

// Paginate 分页查询。
//
// 用法：
//
//	result, err := db.Model[User]().Where("status", 1).Order("id desc").Paginate(1, 20)
//	for _, user := range result.Items { ... }
func (m *Model[T]) Paginate(page, perPage int) (*PaginateResult[T], error) {
	m = m.applyScopes()

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	// 先查总数
	total, err := m.Clone().Count()
	if err != nil {
		return nil, err
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))

	// 查当前页数据
	offset := (page - 1) * perPage
	queryModel := m.Clone()
	queryModel.Limit(perPage)
	queryModel.Offset(offset)

	items, err := queryModel.All()
	if err != nil {
		return nil, err
	}

	return &PaginateResult[T]{
		Items:       items,
		Total:       total,
		PerPage:     perPage,
		CurrentPage: page,
		LastPage:    lastPage,
		HasMore:     page < lastPage,
	}, nil
}

// SimplePaginate 简单分页（不查总数，仅判断是否有下一页）。
func (m *Model[T]) SimplePaginate(page, perPage int) (*PaginateResult[T], error) {
	m = m.applyScopes()

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}

	// 多查一条来判断是否有下一页
	queryModel := m.Clone()
	queryModel.Limit(perPage + 1)
	queryModel.Offset((page - 1) * perPage)

	items, err := queryModel.All()
	if err != nil {
		return nil, err
	}

	hasMore := len(items) > perPage
	if hasMore {
		items = items[:perPage]
	}

	return &PaginateResult[T]{
		Items:       items,
		CurrentPage: page,
		PerPage:     perPage,
		HasMore:     hasMore,
	}, nil
}
