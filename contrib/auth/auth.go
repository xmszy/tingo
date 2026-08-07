// Package auth 提供 HTTP 鉴权中间件。
//
// 设计：零外部依赖，支持 Basic Auth 与 Bearer Token 两种模式。
package auth

import (
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/xmszy/tingo/core"
)

// Basic 返回一个 Basic Auth 中间件，校验用户名/密码。
func Basic(realm string, accounts map[string]string) core.Handler {
	if realm == "" {
		realm = "Authorization Required"
	}
	return func(c *core.Ctx) {
		user, pass, ok := parseBasic(c.Header("Authorization"))
		if !ok || !check(user, pass, accounts) {
			c.G().Header("WWW-Authenticate", `Basic realm="`+realm+`"`)
			c.G().AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{
				"code":    http.StatusUnauthorized,
				"message": "unauthorized",
			})
			return
		}
		c.Next()
	}
}

// Bearer 返回一个 Bearer Token 中间件，仅放行 token 集合中的令牌。
func Bearer(tokens ...string) core.Handler {
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		set[t] = struct{}{}
	}
	return func(c *core.Ctx) {
		tok, ok := parseBearer(c.Header("Authorization"))
		if !ok {
			c.G().AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{
				"code":    http.StatusUnauthorized,
				"message": "missing bearer token",
			})
			return
		}
		if _, ok := set[tok]; !ok {
			c.G().AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{
				"code":    http.StatusUnauthorized,
				"message": "invalid token",
			})
			return
		}
		c.Next()
	}
}

func parseBasic(header string) (user, pass string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(header, prefix) {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(header[len(prefix):])
	if err != nil {
		return "", "", false
	}
	idx := strings.IndexByte(string(decoded), ':')
	if idx < 0 {
		return "", "", false
	}
	return string(decoded[:idx]), string(decoded[idx+1:]), true
}

func parseBearer(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	tok := strings.TrimSpace(header[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}

func check(user, pass string, accounts map[string]string) bool {
	want, ok := accounts[user]
	return ok && want == pass
}
