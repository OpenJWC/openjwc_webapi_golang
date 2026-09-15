package agent

import (
	"context"
	"errors"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/setting"
)

// maxRetryWait 限制单次重试等待，避免长时间占用问答容量。
const maxRetryWait = 10 * time.Second

// defaultRetryWait 是无 Retry-After 提示时的固定退避时间。
const defaultRetryWait = 2 * time.Second

// retryModelOnce 对可重试的瞬时模型失败执行一次有界重试。
func retryModelOnce(ctx context.Context, budget setting.AgentBudget, err error, emitted bool, retry func() error) error {
	if emitted || ctx.Err() != nil {
		return err
	}
	var model *modelError
	if !errors.As(err, &model) || !model.Retry {
		return err
	}
	wait := model.Wait
	if wait <= 0 {
		wait = defaultRetryWait
	}
	wait = min(wait, maxRetryWait)
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < budget.ModelTimeout+wait {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
	}
	return retry()
}
