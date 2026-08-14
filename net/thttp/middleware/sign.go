package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xmszy/tingo/core"
)

/* ------------------------------------------------------------------ */
/* 签名校验中间件                                                        */
/* ------------------------------------------------------------------ */

// SignConfig 是签名校验中间件的配置。
type SignConfig struct {
	// Secret 是签名密钥（HMAC-SHA256 密钥）。
	Secret string
	// TimeTolerance 是时间戳容差，超出视为重放/过期请求。默认 300 秒。
	TimeTolerance time.Duration
	// GetSecret 按 appKey 返回密钥，支持多密钥轮换/多租户。
	// 提供时忽略 Secret 字段。
	GetSecret func(appKey string) (string, bool)
	// AppKeyParam / TimestampParam / NonceParam / SignParam 是签名相关字段名。
	AppKeyParam   string
	TimestampParam string
	NonceParam     string
	SignParam      string
	// IgnorePaths 是不校验签名的路径前缀（如健康检查、登录）。
	IgnorePaths []string
	// ErrCode / ErrMessage 是校验失败的业务码与提示。
	ErrCode    int
	ErrMessage string
}

// DefaultSignConfig 返回默认签名配置。
func DefaultSignConfig() SignConfig {
	return SignConfig{
		TimeTolerance: 300 * time.Second,
		AppKeyParam:   "app_key",
		TimestampParam: "timestamp",
		NonceParam:     "nonce",
		SignParam:      "sign",
		ErrCode:        401,
		ErrMessage:     "invalid signature",
	}
}

// Sign 返回签名校验中间件。
//
// 约定（与绝大多数开放平台一致）：
//  1. 取 app_key / timestamp / nonce 及全部 query 参数；
//  2. 按参数名升序拼接 `k1=v1&k2=v2`（sign 本身不参与）；
//  3. 以 secret 做 HMAC-SHA256，hex 编码后与客户端 sign 比对。
//
// 时间戳超出容差视为过期；nonce 建议业务层配合去重做防重放。
//
//	e.Router().Use(middleware.Sign(func(c *middleware.SignConfig) {
//	    c.Secret = "my-secret"
//	}))
func Sign(opts ...func(*SignConfig)) core.Handler {
	cfg := DefaultSignConfig()
	for _, o := range opts {
		o(&cfg)
	}
	tol := cfg.TimeTolerance
	if tol <= 0 {
		tol = 300 * time.Second
	}

	return func(c *core.Ctx) {
		// 白名单路径直接放行。
		full := c.Path()
		for _, p := range cfg.IgnorePaths {
			if strings.HasPrefix(full, p) {
				c.Next()
				return
			}
		}

		appKey := c.Query(cfg.AppKeyParam)
		tsStr := c.Query(cfg.TimestampParam)
		nonce := c.Query(cfg.NonceParam)
		clientSign := c.Query(cfg.SignParam)

		secret := cfg.Secret
		if cfg.GetSecret != nil {
			s, ok := cfg.GetSecret(appKey)
			if !ok {
				rejectSign(c, cfg)
				return
			}
			secret = s
		}
		if secret == "" || clientSign == "" {
			rejectSign(c, cfg)
			return
		}

		// 时间戳容差校验（防重放 / 过期）。
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			rejectSign(c, cfg)
			return
		}
		now := time.Now().Unix()
		diff := now - ts
		if diff < 0 {
			diff = -diff
		}
		if time.Duration(diff)*time.Second > tol {
			rejectSign(c, cfg)
			return
		}

		// 收集全部参与签名的参数（排除 sign 与 nonce：nonce 仅作为盐值参与，
		// 由 computeSign 统一追加到待签名串末尾，避免重复拼接）。
		params := map[string]string{}
		for k, vs := range c.G().Request.URL.Query() {
			if k == cfg.SignParam || k == cfg.NonceParam {
				continue
			}
			if len(vs) > 0 {
				params[k] = vs[0]
			}
		}

		serverSign := computeSign(params, nonce, secret)
		if !hmac.Equal([]byte(serverSign), []byte(clientSign)) {
			rejectSign(c, cfg)
			return
		}
		c.Next()
	}
}

// computeSign 按参数名升序拼接并做 HMAC-SHA256 签名。
func computeSign(params map[string]string, nonce, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	if nonce != "" {
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString("nonce=")
		b.WriteString(nonce)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(b.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

func rejectSign(c *core.Ctx, cfg SignConfig) {
	c.JSONStatus(cfg.ErrCode, map[string]any{
		"code": cfg.ErrCode,
		"msg":  cfg.ErrMessage,
	})
	c.Abort()
}
