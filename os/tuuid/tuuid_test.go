package tuuid

import (
	"fmt"
	"regexp"
	"testing"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestV4_Format(t *testing.T) {
	s := V4()
	if !uuidRe.MatchString(s) {
		t.Fatalf("V4 格式错误: %q", s)
	}
}

func TestV1Simple_Format(t *testing.T) {
	// V1Simple 也应产出合法 UUID 形态（长度/分隔符一致，版本 bits 置 1）。
	s := V1Simple()
	if !uuidRe.MatchString(s) {
		t.Fatalf("V1Simple 格式错误: %q", s)
	}
	var tl, tm, th, cs, node string
	if _, err := fmt.Sscanf(s, "%8s-%4s-%4s-%4s-%12s", &tl, &tm, &th, &cs, &node); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	// time_hi_and_version 应包含版本位 0x1000（UUID v1 约定）。
	var hi uint16
	if _, err := fmt.Sscanf(th, "%x", &hi); err != nil {
		t.Fatalf("段解析失败: %v", err)
	}
	if hi&0x1000 == 0 {
		t.Fatalf("V1Simple 版本位 0x1000 未置位, 段: %q", th)
	}
}

func TestV1Simple_NotWeakZero(t *testing.T) {
	// 即使 rand.Read 失败（本机不会），fallback 也必须返回非零节点，
	// 不能产生全零弱 UUID。这里确保多次生成不全相等。
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		seen[V1Simple()] = true
	}
	if len(seen) < 50 {
		t.Fatalf("V1Simple 重复率异常高: %d/100", len(seen))
	}
}

func TestShort_Length(t *testing.T) {
	for i := 0; i < 1000; i++ {
		if s := Short(); len(s) != 8 {
			t.Fatalf("Short 长度应为 8, 实际 %d: %q", len(s), s)
		}
	}
}

func TestSID_Length(t *testing.T) {
	if s := SID(); len(s) != 32 {
		t.Fatalf("SID 长度应为 32, 实际 %d: %q", len(s), s)
	}
}
