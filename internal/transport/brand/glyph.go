package brand

// glyphs 是独立绘制的九行窄体点阵，不复用其他项目的字标资产。
var glyphs = map[rune][9]string{
	'O': {".##.", "#..#", "#..#", "#..#", "#..#", "#..#", "#..#", "#..#", ".##."},
	'P': {"###.", "#..#", "#..#", "#..#", "###.", "#...", "#...", "#...", "#..."},
	'E': {"####", "#...", "#...", "#...", "###.", "#...", "#...", "#...", "####"},
	'N': {"#...#", "##..#", "##..#", "#.#.#", "#.#.#", "#..##", "#..##", "#...#", "#...#"},
	'J': {".###", "...#", "...#", "...#", "...#", "...#", "...#", "#..#", ".##."},
	'W': {"#...#", "#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "##.##", ".#.#."},
	'C': {".###", "#...", "#...", "#...", "#...", "#...", "#...", "#...", ".###"},
}

// ink 标识背景、两段字标和投影的像素颜色。
type ink uint8

const (
	empty ink = iota
	muted
	accent
	shadow
)

const canvasWidth = 38

// pixels 将 OpenJWC 点阵与右下投影合成十行窄体像素。
func pixels() [10][canvasWidth]ink {
	var image [10][canvasWidth]ink
	drawPixels(&image, 1, 1, shadow)
	drawPixels(&image, 0, 0, muted)
	return image
}

// drawPixels 按单词分色绘制点阵，JWC 覆盖 OPEN 的默认颜色。
func drawPixels(image *[10][canvasWidth]ink, rowOffset, colOffset int, defaultColor ink) {
	offset := 0
	for index, letter := range "OPENJWC" {
		if index == 4 {
			offset++
		}
		color := defaultColor
		if defaultColor != shadow && index >= 4 {
			color = accent
		}
		for row, line := range glyphs[letter] {
			for col, char := range line {
				if char == '#' {
					image[row+rowOffset][offset+col+colOffset] = color
				}
			}
		}
		offset += len(glyphs[letter][0]) + 1
	}
}
