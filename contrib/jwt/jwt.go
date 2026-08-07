// Package jwt 提供 JWT 认证中间件（基于 golang-jwt/jwt/v5）。
//
// 设计：使用 golang-jwt/jwt/v5 解析与签发令牌。中间件负责校验请求中的
// Bearer Token，并将声明写入上下文供后续 handler 读取。
package jwt

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xmszy/tingo/core"
)

// Claims 是默认声明类型，业务可自定义并嵌入 jwt.RegisteredClaims。
type Claims = jwt.RegisteredClaims

// Config 是 JWT 中间件配置。
type Config struct {
	// Secret 签名密钥（HS256）。
	Secret string
	// Lookup 从何处取令牌，默认 "header:Authorization"。
	Lookup string
	// AuthScheme 令牌前缀，默认 "Bearer"。
	AuthScheme string
	// Exclude 跳过校验的路径前缀列表（如登录接口）。
	Exclude []string
	// Unauthorized 自定义未授权响应。
	Unauthorized func(c *core.Ctx, err error)
}

// FromContext 返回上下文中解析出的声明（需先经 Middleware 校验）。
// 返回 nil 表示未认证。
func FromContext(c *core.Ctx) jwt.Claims {
	if v, ok := c.G().Get("jwt_claims"); ok {
		if cl, ok := v.(jwt.Claims); ok {
			return cl
		}
	}
	return nil
}

// NewToken 使用 HS256 签发一个令牌。
func NewToken(secret string, subject string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}

// Middleware 返回 JWT 校验中间件。
func Middleware(cfg Config) core.Handler {
	if cfg.AuthScheme == "" {
		cfg.AuthScheme = "Bearer"
	}
	key := []byte(cfg.Secret)
	return func(c *core.Ctx) {
		for _, p := range cfg.Exclude {
			if strings.HasPrefix(c.Path(), p) {
				c.Next()
				return
			}
		}
		tokStr, err := extractToken(c, cfg)
		if err != nil {
			unauthorized(c, cfg, err)
			return
		}
		claims := &jwt.RegisteredClaims{}
		_, err = jwt.ParseWithClaims(tokStr, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return key, nil
		})
		if err != nil {
			unauthorized(c, cfg, err)
			return
		}
		c.G().Set("jwt_claims", claims)
		c.Next()
	}
}

func extractToken(c *core.Ctx, cfg Config) (string, error) {
	auth := c.Header("Authorization")
	if auth == "" {
		return "", jwt.ErrTokenMalformed
	}
	prefix := cfg.AuthScheme + " "
	if strings.HasPrefix(auth, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(auth, prefix)), nil
	}
	return "", jwt.ErrTokenMalformed
}

func unauthorized(c *core.Ctx, cfg Config, err error) {
	if cfg.Unauthorized != nil {
		cfg.Unauthorized(c, err)
		return
	}
	c.G().AbortWithStatusJSON(http.StatusUnauthorized, map[string]any{
		"code":    http.StatusUnauthorized,
		"message": "invalid or missing token",
	})
}
