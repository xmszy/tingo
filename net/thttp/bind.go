package thttp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xmszy/tingo/core"
	"github.com/xmszy/tingo/os/tvalid"
)

// BindAndValid 将请求体/查询参数绑定到 req 指针，并用 tvalid 校验。
// 返回校验错误时，直接将 400 响应写入 c 并返回 true（调用方应中止）。
//
// scene 为可选的场景名（验证场景）：传入时按场景规则校验，
// 覆盖 req 上 tdb tag 的 scene 约束。空字符串表示不启用场景。
//
// 用法：
//
//	var req LoginReq
//	if thttp.BindAndValid(c, &req) {
//	    return
//	}
//	// 按场景校验：
//	if thttp.BindAndValid(c, &req, "create") {
//	    return
//	}
func BindAndValid(c *core.Ctx, req any, scene ...string) bool {
	g := (*gin.Context)(c)
	if err := g.ShouldBind(req); err != nil {
		WriteError(g, http.StatusBadRequest, 1, err)
		return true
	}
	if err := validateStructWithScene(req, scene...); err != nil {
		WriteError(g, http.StatusBadRequest, 1, err)
		return true
	}
	return false
}

// validateStructWithScene 按场景校验结构体：有场景时走 CheckStructWithScene，否则走 CheckStruct。
func validateStructWithScene(req any, scene ...string) error {
	if len(scene) > 0 && scene[0] != "" {
		return tvalid.CheckStructWithScene(req, scene[0])
	}
	return tvalid.CheckStruct(req)
}

// WriteData 写入标准 JSON 响应：{code, message, data}。
func WriteData(g *gin.Context, httpStatus, code int, message string, data any) {
	g.JSON(httpStatus, gin.H{
		"code":    code,
		"message": message,
		"data":    data,
	})
}

// WriteOK 写入成功响应（http 200，业务 code 0）。
func WriteOK(g *gin.Context, data any) {
	WriteData(g, http.StatusOK, 0, "ok", data)
}

// WritePage 写入分页列表响应。
// list 为当页数据，total 为总记录数，page/size 为当前页码与每页大小。
func WritePage(g *gin.Context, list any, total int64, page, size int) {
	WriteData(g, http.StatusOK, 0, "ok", gin.H{
		"list":  list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// WriteError 将错误标准化为 JSON 响应。
// 若 err 是 *tvalid.Errors（校验失败），返回 200 + 业务码 1 + 首条错误信息；
// 否则按传入 httpStatus 返回业务码（默认 1）。
func WriteError(g *gin.Context, httpStatus, code int, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	g.JSON(httpStatus, gin.H{
		"code":    code,
		"message": msg,
		"data":    nil,
	})
}
