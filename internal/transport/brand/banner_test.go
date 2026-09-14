package brand

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestWordmarkFitsTerminalWithoutBreakingCells 验证大小字标与窄屏降级均不越过终端列宽。
func TestWordmarkFitsTerminalWithoutBreakingCells(t *testing.T) {
	for _, width := range []int{0, 6, canvasWidth - 1, canvasWidth, canvasWidth*2 - 1, canvasWidth * 2, 120} {
		for _, colored := range []bool{false, true} {
			text := Render(width, colored)
			for _, line := range strings.Split(text, "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("字标越界: width=%d line=%q", width, line)
				}
			}
			if !colored && strings.Contains(text, "\x1b") {
				t.Fatal("单色字标包含控制序列")
			}
			if width >= canvasWidth && len(strings.Split(text, "\n")) != 5 {
				t.Fatal("块状字标高度错误")
			}
		}
	}
}

// TestWordmarkHasShadowAndTwoToneColor 验证独立点阵包含立体投影与两段配色。
func TestWordmarkHasShadowAndTwoToneColor(t *testing.T) {
	plain := Render(43, false)
	if !strings.Contains(plain, "░") || !strings.ContainsAny(plain, "█▀▄") {
		t.Fatal("字标缺少块状字形或投影")
	}
	colored := Render(43, true)
	if !strings.Contains(colored, "38;5;245") || !strings.Contains(colored, "38;5;86") {
		t.Fatal("字标缺少双色区分")
	}
	t.Log("\n" + plain)
}

// TestRedirectedOutputDoesNotContainBrandArt 验证帮助或退出输出到非终端时不追加字标。
func TestRedirectedOutputDoesNotContainBrandArt(t *testing.T) {
	var out bytes.Buffer
	Print(&out, false)
	Print(&out, true)
	if out.Len() != 0 {
		t.Fatal("管道输出被装饰污染")
	}
}
