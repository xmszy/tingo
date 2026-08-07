// Package metric 提供轻量级 Prometheus 风格指标中间件（零外部依赖）。
//
// 提供请求观测能力，用标准库实现，不引入 otel/prometheus client。
// 中间件零侵入采集 QPS、状态码分布、请求延迟分桶、
// 错误率，并通过 /metrics 端点暴露 Prometheus 文本格式。
package metric

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xmszy/tingo/core"
)

// 延迟分桶边界（秒）。
var latencyBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// Collector 累积指标数据，所有字段均原子操作，可在请求热路径无锁更新。
type Collector struct {
	total        uint64
	statusCodes  map[int]*uint64
	statusMu     sync.RWMutex
	latencyCount []uint64
	latencySum   uint64 // 纳秒累计
	startTime    time.Time
}

// New 创建 Collector。
func New() *Collector {
	return &Collector{
		statusCodes: map[int]*uint64{},
		latencyCount: make([]uint64, len(latencyBuckets)+1),
		startTime:   time.Now(),
	}
}

var defaultCollector = New()

// Default 返回全局 Collector。
func Default() *Collector { return defaultCollector }

func (c *Collector) observe(status int, dur time.Duration) {
	atomic.AddUint64(&c.total, 1)
	atomic.AddUint64(&c.latencySum, uint64(dur.Nanoseconds()))

	c.statusMu.RLock()
	counter, ok := c.statusCodes[status]
	c.statusMu.RUnlock()
	if !ok {
		c.statusMu.Lock()
		if counter, ok = c.statusCodes[status]; !ok {
			counter = new(uint64)
			c.statusCodes[status] = counter
		}
		c.statusMu.Unlock()
	}
	atomic.AddUint64(counter, 1)

	sec := dur.Seconds()
	idx := len(latencyBuckets)
	for i, b := range latencyBuckets {
		if sec <= b {
			idx = i
			break
		}
	}
	atomic.AddUint64(&c.latencyCount[idx], 1)
}

// Snapshot 导出即时指标快照。
func (c *Collector) Snapshot() (total uint64, byStatus map[int]uint64, latencyHist []uint64, sumNs uint64, uptime time.Duration) {
	total = atomic.LoadUint64(&c.total)
	byStatus = make(map[int]uint64, len(c.statusCodes))
	c.statusMu.RLock()
	for k, v := range c.statusCodes {
		byStatus[k] = atomic.LoadUint64(v)
	}
	c.statusMu.RUnlock()
	latencyHist = make([]uint64, len(c.latencyCount))
	for i := range c.latencyCount {
		latencyHist[i] = atomic.LoadUint64(&c.latencyCount[i])
	}
	sumNs = atomic.LoadUint64(&c.latencySum)
	uptime = time.Since(c.startTime)
	return
}

// Middleware 返回指标采集中间件。
func Middleware(c *Collector) core.Handler {
	if c == nil {
		c = defaultCollector
	}
	return func(ctx *core.Ctx) {
		start := time.Now()
		ctx.Next()
		c.observe(ctx.Res().Status(), time.Since(start))
	}
}

// Handler 返回 /metrics 端点处理函数（Prometheus 文本格式）。
func Handler(c *Collector) func(*core.Ctx) {
	if c == nil {
		c = defaultCollector
	}
	return func(ctx *core.Ctx) {
		ctx.G().Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		ctx.G().String(http.StatusOK, c.Render())
	}
}

// Render 输出 Prometheus 文本格式。
func (c *Collector) Render() string {
	total, byStatus, hist, sumNs, uptime := c.Snapshot()
	var b strings.Builder
	b.WriteString("# HELP tingo_http_requests_total Total HTTP requests.\n")
	b.WriteString("# TYPE tingo_http_requests_total counter\n")
	b.WriteString(fmt.Sprintf("tingo_http_requests_total %d\n", total))

	b.WriteString("# HELP tingo_http_request_duration_seconds Histogram of request latency.\n")
	b.WriteString("# TYPE tingo_http_request_duration_seconds histogram\n")
	for i, bound := range latencyBuckets {
		b.WriteString(fmt.Sprintf("tingo_http_request_duration_seconds_bucket{le=\"%s\"} %d\n", strconv.FormatFloat(bound, 'g', -1, 64), hist[i]))
	}
	b.WriteString(fmt.Sprintf("tingo_http_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", hist[len(hist)-1]))
	b.WriteString(fmt.Sprintf("tingo_http_request_duration_seconds_sum %s\n", floatToStr(float64(sumNs)/1e9)))
	b.WriteString(fmt.Sprintf("tingo_http_request_duration_seconds_count %d\n", total))

	b.WriteString("# HELP tingo_http_requests_by_code Requests grouped by status code.\n")
	b.WriteString("# TYPE tingo_http_requests_by_code counter\n")
	codes := make([]int, 0, len(byStatus))
	for code := range byStatus {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	for _, code := range codes {
		b.WriteString(fmt.Sprintf("tingo_http_requests_by_code{code=\"%d\"} %d\n", code, byStatus[code]))
	}

	b.WriteString("# HELP tingo_uptime_seconds Process uptime.\n")
	b.WriteString("# TYPE tingo_uptime_seconds gauge\n")
	b.WriteString(fmt.Sprintf("tingo_uptime_seconds %s\n", floatToStr(uptime.Seconds())))

	return b.String()
}

func floatToStr(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
