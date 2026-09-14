package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// crawlRequest 是示例程序接受的 OpenJWC NDJSON v1 请求。
type crawlRequest struct {
	Version  int    `json:"version"`
	Type     string `json:"type"`
	RunID    string `json:"run_id"`
	Source   string `json:"source"`
	Since    string `json:"since"`
	MaxPages int    `json:"max_pages"`
}

// crawlEvent 是示例程序逐行输出的协议事件。
type crawlEvent struct {
	Version int          `json:"version"`
	Type    string       `json:"type"`
	Scanned *int         `json:"scanned,omitempty"`
	Notice  *crawlNotice `json:"notice,omitempty"`
}

// crawlNotice 是示例资讯，主程序仍会重新校验和生成 ID。
type crawlNotice struct {
	Label       string `json:"label"`
	Title       string `json:"title"`
	PublishedAt string `json:"published_at"`
	DetailURL   string `json:"detail_url"`
	IsPage      bool   `json:"is_page"`
	Content     string `json:"content"`
}

// main 运行一个仅用于本地协议验收的单条资讯外部爬虫。
func main() {
	label := flag.String("label", "本地测试", "notice label")
	flag.Parse()
	var request crawlRequest
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 65537))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || request.Version != 1 || request.Type != "crawl.request" || request.Source == "" {
		log.Fatal("invalid OpenJWC crawl request")
	}
	encoder := json.NewEncoder(os.Stdout)
	write(encoder, crawlEvent{Version: 1, Type: "progress", Scanned: number(1)})
	item := crawlNotice{Label: *label, Title: "外部爬虫协议测试", PublishedAt: time.Now().UTC().Format(time.RFC3339), DetailURL: "https://example.edu/openjwc-crawler-test", IsPage: true, Content: "该资讯由 tools/crawlmock 生成，仅用于临时数据库测试。"}
	write(encoder, crawlEvent{Version: 1, Type: "notice", Notice: &item})
	write(encoder, crawlEvent{Version: 1, Type: "completed", Scanned: number(1)})
}

// number 返回事件可选累计字段所需的独立指针。
func number(value int) *int { return &value }

// write 编码单个 JSON 行并在失败时以非零状态退出。
func write(encoder *json.Encoder, event crawlEvent) {
	if err := encoder.Encode(event); err != nil {
		fmt.Fprintln(os.Stderr, "encode event failed")
		os.Exit(1)
	}
}
