// Package captcha 提供图形验证码中间件。
//
// 设计：基于 dchest/captcha 生成与校验，零业务侵入：提供两个 handler——
// Serve（输出图片）与 Verify（校验），可直接挂载到路由。
package captcha

import (
	"net/http"
	"strings"

	"github.com/dchest/captcha"
	"github.com/xmszy/tingo/core"
)

// Serve 返回一个 handler，按 URL 末段作为验证码 id 输出 PNG 图片。
//
//	// GET /captcha/:id
//	r.GET("/captcha/:id", captcha.Serve)
//
// 支持 ?reload=1 重新生成。
func Serve(c *core.Ctx) {
	id := c.Param("id")
	if id == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	c.G().Header("Content-Type", "image/png")
	if c.Query("reload") != "" {
		captcha.Reload(id)
	}
	if err := captcha.WriteImage(c.Res(), id, 240, 80); err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
}

// New 生成一个新的验证码 id 并以 JSON 返回（含图片 URL 前缀）。
//
//	// GET /captcha/new
//	r.GET("/captcha/new", captcha.New("/captcha"))
func New(urlPrefix string) core.Handler {
	prefix := strings.TrimRight(urlPrefix, "/")
	return func(c *core.Ctx) {
		id := captcha.New()
		c.JSON(map[string]any{
			"captcha_id": id,
			"url":        prefix + "/" + id,
		})
	}
}

// Verify 返回一个校验中间件，从表单/查询读取 captcha_id 与 captcha_value，
// 校验失败返回 400。校验成功后通过 core.Ctx 的 gin 上下文传递 captcha_id。
func Verify(idField, valField string) core.Handler {
	if idField == "" {
		idField = "captcha_id"
	}
	if valField == "" {
		valField = "captcha_value"
	}
	return func(c *core.Ctx) {
		id := c.Post(idField)
		if id == "" {
			id = c.Query(idField)
		}
		val := c.Post(valField)
		if val == "" {
			val = c.Query(valField)
		}
		if id == "" || val == "" {
			c.G().AbortWithStatusJSON(http.StatusBadRequest, map[string]any{
				"code":    http.StatusBadRequest,
				"message": "captcha required",
			})
			return
		}
		if !captcha.VerifyString(id, val) {
			c.G().AbortWithStatusJSON(http.StatusBadRequest, map[string]any{
				"code":    http.StatusBadRequest,
				"message": "captcha mismatch",
			})
			return
		}
		c.Next()
	}
}
