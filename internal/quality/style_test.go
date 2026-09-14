package quality

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// TestHandwrittenGoFilesFollowCommentAndSizeRules 验证手写文件行数及所有函数、结构体的一行中文说明。
func TestHandwrittenGoFilesFollowCommentAndSizeRules(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains("\n"+string(content), "\n// Code generated ") {
			return nil
		}
		if strings.Count(string(content), "\n") > 300 {
			t.Errorf("%s 超过 300 行", path)
		}
		checkDeclarations(t, path, content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// checkDeclarations 检查声明说明与函数体内禁止注释的约束。
func checkDeclarations(t *testing.T, path string, content []byte) {
	t.Helper()
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, path, content, parser.ParseComments)
	if err != nil {
		t.Error(err)
		return
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			checkChineseDoc(t, path, declaration.Name.Name, declaration.Doc)
			if declaration.Body != nil {
				for _, comment := range file.Comments {
					if comment.Pos() > declaration.Body.Pos() && comment.End() < declaration.Body.End() {
						t.Errorf("%s:%d 函数体内不允许注释", path, positions.Position(comment.Pos()).Line)
					}
				}
			}
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); !ok {
					continue
				}
				doc := typeSpec.Doc
				if doc == nil {
					doc = declaration.Doc
				}
				checkChineseDoc(t, path, typeSpec.Name.Name, doc)
			}
		}
	}
}

// checkChineseDoc 要求说明仅占一行，包含中文并以声明标识符开头。
func checkChineseDoc(t *testing.T, path string, name string, doc *ast.CommentGroup) {
	t.Helper()
	if doc == nil || len(doc.List) != 1 {
		t.Errorf("%s 的 %s 必须有一行说明", path, name)
		return
	}
	text := strings.TrimSpace(strings.TrimPrefix(doc.List[0].Text, "//"))
	if !strings.HasPrefix(text, name+" ") {
		t.Errorf("%s 的注释必须以 %s 开头", path, name)
	}
	if strings.Contains(text, "\n") || !strings.ContainsFunc(text, func(char rune) bool { return unicode.Is(unicode.Han, char) }) {
		t.Errorf("%s 的 %s 注释必须为一行中文", path, name)
	}
}
