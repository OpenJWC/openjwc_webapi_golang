package crawler

import (
	"golang.org/x/net/html"
	"strings"
)

// attribute 返回节点的指定 HTML 属性。
func attribute(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// hasClass 按独立 class token 匹配，避免误选相似名称。
func hasClass(node *html.Node, name string) bool {
	for _, value := range strings.Fields(attribute(node, "class")) {
		if value == name {
			return true
		}
	}
	return false
}

// descendants 收集满足条件的后代节点，输入来自有界 HTML 响应。
func descendants(node *html.Node, match func(*html.Node) bool) []*html.Node {
	var result []*html.Node
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if match(current) {
			result = append(result, current)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return result
}

// nodeText 提取可见正文并忽略脚本、样式及模板。
func nodeText(node *html.Node) string {
	var output strings.Builder
	var visit func(*html.Node)
	visit = func(current *html.Node) {
		if current.Type == html.ElementNode && (current.Data == "script" || current.Data == "style" || current.Data == "noscript") {
			return
		}
		if current.Type == html.TextNode {
			output.WriteString(current.Data)
			output.WriteByte(' ')
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return strings.Join(strings.Fields(output.String()), " ")
}
