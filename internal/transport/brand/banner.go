package brand

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"
)

// Print 仅向交互终端输出字标，重定向和禁用时不写装饰。
func Print(out io.Writer, disabled bool) {
	file, ok := out.(*os.File)
	if disabled || !ok || !term.IsTerminal(file.Fd()) {
		return
	}
	width, _, err := term.GetSize(file.Fd())
	if err != nil || width <= 0 {
		return
	}
	fmt.Fprintln(out)
	if width < 11 {
		fmt.Fprintln(out, Render(width, false))
		return
	}
	if os.Getenv("TERM") == "dumb" {
		fmt.Fprintln(out, "OpenJWC")
		return
	}
	colored := ColorEnabled(out)
	text := Render(max(0, width-4), colored)
	for _, line := range strings.Split(text, "\n") {
		fmt.Fprintln(out, "  "+line)
	}
	fmt.Fprintln(out)
}

// ColorEnabled 判断输出端是否明确支持颜色，并遵守 NO_COLOR。
func ColorEnabled(out io.Writer) bool {
	file, ok := out.(*os.File)
	return ok && term.IsTerminal(file.Fd()) && os.Getenv("NO_COLOR") == "" && (strings.Contains(os.Getenv("TERM"), "256color") || os.Getenv("COLORTERM") != "")
}

// Render 按可用列数选择双宽或紧凑块字，窄屏退回普通名称。
func Render(width int, colored bool) string {
	if width < canvasWidth {
		if width <= 0 {
			return ""
		}
		return "OpenJWC"[:min(width, len("OpenJWC"))]
	}
	scale := 1
	if width >= canvasWidth*2 {
		scale = 2
	}
	image := pixels()
	var result strings.Builder
	for row := 0; row < 10; row += 2 {
		if row > 0 {
			result.WriteByte('\n')
		}
		for col := 0; col < canvasWidth; col++ {
			result.WriteString(strings.Repeat(cell(image[row][col], image[row+1][col], colored), scale))
		}
	}
	return result.String()
}

// cell 用上下半块编码两个像素，单色模式保留字形并以浅块表示投影。
func cell(upper, lower ink, colored bool) string {
	if !colored {
		top, bottom := upper == muted || upper == accent, lower == muted || lower == accent
		switch {
		case top && bottom:
			return "█"
		case top:
			return "▀"
		case bottom:
			return "▄"
		case upper == shadow || lower == shadow:
			return "░"
		default:
			return " "
		}
	}
	colors := [4]int{0, 245, 86, 238}
	if upper == empty && lower == empty {
		return " "
	}
	if upper == empty {
		return fmt.Sprintf("\x1b[38;5;%dm▄\x1b[0m", colors[lower])
	}
	if lower == empty {
		return fmt.Sprintf("\x1b[38;5;%dm▀\x1b[0m", colors[upper])
	}
	if upper == lower {
		return fmt.Sprintf("\x1b[38;5;%dm█\x1b[0m", colors[upper])
	}
	return fmt.Sprintf("\x1b[38;5;%d;48;5;%dm▀\x1b[0m", colors[upper], colors[lower])
}
