package tapp

import (
	"net/http"

	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/database/tdb"
)

/* ------------------------------------------------------------------ */
/* 分页响应桥接                                                          */
/* ------------------------------------------------------------------ */

// PageData 是标准的分页响应结构。
//
// 字段顺序：total、per_page、current_page、last_page、list。
type PageData struct {
	// Total 是符合条件的总记录数。
	Total int64 `json:"total"`
	// PerPage 是每页条数。
	PerPage int `json:"per_page"`
	// CurrentPage 是当前页码（从 1 开始）。
	CurrentPage int `json:"current_page"`
	// LastPage 是最后一页页码。
	LastPage int `json:"last_page"`
	// List 是当前页数据。
	List any `json:"list"`
}

// NewPageData 由 tdb.PaginateResult 构造标准分页响应（零转换桥接）。
//
// 用法：
//
//	pr, _ := db.Model[User]().Paginate(page, size)
//	return ctrl.SuccessPage(c, tapp.NewPageData(pr))
func NewPageData[T any](pr *tdb.PaginateResult[T]) *PageData {
	return &PageData{
		Total:       pr.Total,
		PerPage:     pr.PerPage,
		CurrentPage: pr.CurrentPage,
		LastPage:    pr.LastPage,
		List:        pr.Items,
	}
}

// SuccessPage 输出分页成功响应。
func (ctrl *Controller) SuccessPage(c *core.Ctx, page *PageData, msg ...string) error {
	m := "success"
	if len(msg) > 0 {
		m = msg[0]
	}
	c.JSONStatus(http.StatusOK, &Result{Code: CodeSuccess, Msg: m, Data: page})
	return nil
}

// SuccessPageData 是 SuccessPage 的别名，语义更明确（当分页结果来自非 tdb 来源时）。
func (ctrl *Controller) SuccessPageData(c *core.Ctx, page *PageData, msg ...string) error {
	return ctrl.SuccessPage(c, page, msg...)
}
