package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// module 描述 Go 工具链报告的依赖模块元数据。
type module struct {
	Path    string
	Version string
	Dir     string
	Main    bool
}

// dependency 描述构建依赖对应的模块。
type dependency struct{ Module *module }

// main 从实际二进制依赖生成可嵌入的许可证材料。
func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// generate 收集运行时依赖许可证，缺失时失败而不是静默遗漏。
func generate() error {
	output, err := exec.Command("go", "list", "-deps", "-json", "./cmd/openjwc").Output()
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	modules := make(map[string]module)
	for {
		var item dependency
		err := decoder.Decode(&item)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if item.Module != nil && !item.Module.Main {
			modules[item.Module.Path] = *item.Module
		}
	}
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	var content strings.Builder
	content.WriteString("第三方许可证；由 go run ./tools/licenses 从当前运行时依赖生成。\n")
	for _, name := range names {
		entry := modules[name]
		files, err := os.ReadDir(entry.Dir)
		if err != nil {
			return err
		}
		found := false
		for _, file := range files {
			upper := strings.ToUpper(file.Name())
			if file.IsDir() || (!strings.HasPrefix(upper, "LICENSE") && !strings.HasPrefix(upper, "COPYING")) {
				continue
			}
			license, err := os.ReadFile(filepath.Join(entry.Dir, file.Name()))
			if err != nil {
				return err
			}
			fmt.Fprintf(&content, "\n===== %s %s / %s =====\n%s\n", entry.Path, entry.Version, file.Name(), strings.TrimRight(string(license), "\r\n"))
			found = true
		}
		if !found {
			return fmt.Errorf("依赖缺少顶层许可证文件: %s", entry.Path)
		}
	}
	return os.WriteFile("internal/license/third_party.txt", []byte(content.String()), 0644)
}
