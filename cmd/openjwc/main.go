package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	_ "time/tzdata"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/transport/cli"
)

// main 仅负责进程信号、标准流与 CLI 退出码。
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	code, err := cli.Run(ctx, os.Args[1:], os.Stdout)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
