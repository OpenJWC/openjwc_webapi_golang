package digest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/report"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/agent"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// Repository 定义日报状态与当天资讯集合的持久化能力。
type Repository interface {
	DailySources(ctx context.Context, day string) ([]string, error)
	DailyStatus(ctx context.Context, day string) (report.Status, error)
	SaveDaily(ctx context.Context, day string, status report.Status, content string, sourceCount int) error
}

// Service 将当日资讯分批总结后合成日报，重复触发不会重复发布。
type Service struct {
	repository Repository
	settings   setting.Reader
	agent      agent.Chat
	slot       chan struct{}
}

// New 创建独立会话的日报服务，并与问答共享 Agent 全局预算。
func New(repository Repository, settings setting.Reader, chat agent.Chat) *Service {
	return &Service{repository: repository, settings: settings, agent: chat, slot: make(chan struct{}, 1)}
}

// Generate 对指定日期执行可重试的生成流程，失败不发布半成品。
func (service *Service) Generate(ctx context.Context, day string) (err error) {
	if _, err = time.Parse("2006-01-02", day); err != nil {
		return fmt.Errorf("日报日期必须为 YYYY-MM-DD")
	}
	select {
	case service.slot <- struct{}{}:
	default:
		return fmt.Errorf("日报任务已在运行")
	}
	defer func() { <-service.slot }()
	status, err := service.repository.DailyStatus(ctx, day)
	if err != nil {
		return err
	}
	if status == report.Completed {
		return nil
	}
	ids, err := service.repository.DailySources(ctx, day)
	if err != nil {
		return err
	}
	if len(ids) > 100 {
		return fmt.Errorf("当日资讯超过 100 条，需扩大已验证的日报预算后重试")
	}
	if err = service.repository.SaveDaily(ctx, day, report.Running, "", len(ids)); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = service.repository.SaveDaily(cleanup, day, report.Failed, "", len(ids))
		}
	}()
	if len(ids) == 0 {
		return service.repository.SaveDaily(ctx, day, report.Completed, "当日没有已收录的资讯。", 0)
	}
	settings, err := service.settings.Settings(ctx)
	if err != nil {
		return err
	}
	var parts []string
	for start := 0; start < len(ids); start += 8 {
		part, err := service.agent.Answer(ctx, agent.Request{Query: fmt.Sprintf("这是 %s 日报的一批资讯。%s\n请逐条阅读所选资讯，给出有证据的简洁摘要。", day, settings["daily_prompt"]), NoticeIDs: ids[start:min(len(ids), start+8)]})
		if err != nil {
			return err
		}
		parts = append(parts, part)
	}
	content := strings.Join(parts, "\n\n")
	if len(parts) > 1 {
		request := agent.Request{Query: fmt.Sprintf("请根据以下 %s 分批资讯摘要合并成一篇日报，保留来源引用，去重且不要添加没有证据的事实。\n%s", day, content)}
		if err = request.Validate(); err != nil {
			return fmt.Errorf("合成日报上下文超过预算: %w", err)
		}
		content, err = service.agent.Answer(ctx, request)
		if err != nil {
			return err
		}
	}
	return service.repository.SaveDaily(ctx, day, report.Completed, content, len(ids))
}
