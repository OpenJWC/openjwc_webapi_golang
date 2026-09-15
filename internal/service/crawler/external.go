package crawler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/crawl"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/notice"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// Program 描述经过配置层校验的外部爬虫命令，不经过 shell。
type Program struct {
	Name string
	Path string
	Args []string
}

// External 运行一个受任务上下文约束的 NDJSON 爬虫进程。
type External struct {
	program    Program
	settings   setting.Reader
	repository Repository
}

// Valid 检查进程边界所需的来源名、规范绝对路径和参数上限。
func (program Program) Valid() bool {
	if !crawl.ValidSourceName(program.Name) || crawl.BuiltinSourceName(program.Name) || filepath.Clean(program.Path) != program.Path || !filepath.IsAbs(program.Path) || len(program.Path) > 4096 || len(program.Args) > 32 {
		return false
	}
	for _, argument := range program.Args {
		if len(argument) > 4096 || strings.ContainsAny(argument, "\x00\n\r") {
			return false
		}
	}
	return !strings.ContainsAny(program.Path, "\x00\n\r")
}

// NewExternal 创建外部爬虫适配器并复制固定参数。
func NewExternal(program Program, settings setting.Reader, repository Repository) *External {
	program.Args = append([]string(nil), program.Args...)
	return &External{program: program, settings: settings, repository: repository}
}

// Name 返回部署配置中的爬虫名称，供具名调度选择。
func (external *External) Name() string { return external.program.Name }

// RunObserved 启动一次进程，持续校验进度和资讯，最终状态通过同一观察端口持久化。
func (external *External) RunObserved(ctx context.Context, observe Observer) (int, error) {
	if !external.program.Valid() {
		return 0, fmt.Errorf("外部爬虫程序配置无效")
	}
	values, err := external.settings.Settings(ctx)
	if err != nil {
		return 0, err
	}
	maxPages, err := validatedNumber(values, "crawler_max_pages")
	if err != nil {
		return 0, err
	}
	days, err := validatedNumber(values, "crawler_days_gap")
	if err != nil {
		return 0, err
	}
	source := crawl.Source{Name: external.program.Name, State: crawl.Running, StartedAt: time.Now().UTC()}
	if observe != nil {
		if err = observe(ctx, source); err != nil {
			return 0, err
		}
	}
	request := protocolRequest{Version: 1, Type: "crawl.request", RunID: fmt.Sprintf("%d", time.Now().UnixNano()), Source: external.program.Name, Since: time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339), MaxPages: maxPages}
	count, detail, err := external.execute(ctx, request, &source, observe)
	source.FinishedAt = time.Now().UTC()
	source.State = crawl.Completed
	source.Detail = safeDetail(detail, "")
	if err != nil {
		source.State = crawl.Failed
		source.Detail = safeDetail(detail, "外部爬虫执行失败")
	}
	if ctx.Err() != nil {
		source.State = crawl.Canceled
		source.Detail = "任务已取消"
		err = ctx.Err()
	}
	if observe != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		observeErr := observe(cleanup, source)
		cancel()
		if observeErr != nil {
			return count, errors.Join(err, observeErr)
		}
	}
	return count, err
}

// validatedNumber 复用设置边界并返回外部协议需要的整数。
func validatedNumber(values map[string]string, key string) (int, error) {
	if err := setting.Validate(key, values[key]); err != nil {
		return 0, err
	}
	number, err := strconv.Atoi(values[key])
	if err != nil {
		return 0, fmt.Errorf("解析设置 %s: %w", key, err)
	}
	return number, nil
}

// execute 读取有界事件流，主程序而非子进程拥有资讯构造与 SQLite 写入。
func (external *External) execute(ctx context.Context, request protocolRequest, source *crawl.Source, observe Observer) (int, string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return 0, "", err
	}
	command := exec.CommandContext(ctx, external.program.Path, external.program.Args...)
	command.Stdin = bytes.NewReader(append(payload, '\n'))
	command.Stderr = io.Discard
	command.Dir = "/"
	command.Env = []string{"LANG=C.UTF-8", "PATH=/usr/bin:/bin"}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	command.WaitDelay = 2 * time.Second
	command.Cancel = func() error { return killProcessGroup(command) }
	stdout, err := command.StdoutPipe()
	if err != nil {
		return 0, "", err
	}
	if err = command.Start(); err != nil {
		return 0, "", fmt.Errorf("启动外部爬虫 %s: %w", external.program.Name, err)
	}
	limited := &io.LimitedReader{R: stdout, N: (32 << 20) + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, detail, parseErr := external.readEvents(ctx, scanner, source, observe)
	if parseErr != nil {
		_ = killProcessGroup(command)
	}
	waitErr := command.Wait()
	if ctx.Err() != nil {
		return count, detail, ctx.Err()
	}
	if limited.N <= 0 {
		return count, detail, fmt.Errorf("外部爬虫输出超过 32 MiB")
	}
	if parseErr != nil {
		return count, detail, parseErr
	}
	if waitErr != nil {
		return count, detail, fmt.Errorf("外部爬虫 %s 非正常退出", external.program.Name)
	}
	return count, detail, nil
}

// killProcessGroup 取消整个独立进程组，避免外部程序遗留自己启动的子进程。
func killProcessGroup(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// safeDetail 清除控制字符并限制外部程序可进入管理界面的说明。
func safeDetail(value, fallback string) string {
	value = strings.Map(func(char rune) rune {
		if unicode.IsControl(char) || unicode.Is(unicode.Cf, char) {
			return ' '
		}
		return char
	}, strings.TrimSpace(value))
	characters := []rune(value)
	if len(characters) > 512 {
		value = string(characters[:512])
	}
	if value == "" {
		return fallback
	}
	return value
}

// saveExternalNotice 校验外部输入并用既有领域构造器保存。
func (external *External) saveExternalNotice(ctx context.Context, input protocolNotice) error {
	create, err := input.createInput()
	if err != nil {
		return err
	}
	item, err := notice.New(create)
	if err != nil {
		return err
	}
	return external.repository.Save(ctx, item)
}
