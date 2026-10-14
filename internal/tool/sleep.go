package tool

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type SleepInput struct {
	Duration       string `json:"duration" jsonschema:"required,description=睡眠时长，以 go duration string 表示"`
	ExternalWakeUp bool   `json:"external_wakeup" jsonschema:"required,description=是否可被外部事件中断，如果被吵到了可以关"`
}

type SleepOutput struct {
	Awake   bool      `json:"awake"`
	Now     time.Time `json:"now"`
	Comment string    `json:"comment"`
}

// SleepTool blocks the current turn until the timer fires, ctx is cancelled, or
// (when the caller allows it) an external event arrives on ch with a comment.
func SleepTool(ch <-chan string) func(context.Context, *SleepInput) (*SleepOutput, error) {
	return func(ctx context.Context, in *SleepInput) (*SleepOutput, error) {
		dur, err := time.ParseDuration(in.Duration)
		if err != nil {
			return nil, err
		}

		if dur > 3*time.Hour {
			return nil, fmt.Errorf("duration too long (need <= 3h)")
		}

		timer := time.NewTimer(dur)
		defer timer.Stop()

		// A nil channel blocks forever, so this disables the case when external wake-up is not allowed.
		var wakeCh <-chan string
		if in.ExternalWakeUp {
			wakeCh = ch
		}

		var comment string
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		case comment = <-wakeCh:
		}

		now := time.Now().In(beijing)
		return &SleepOutput{Awake: true, Now: now, Comment: comment}, nil
	}
}

func SleepErrorHandler(ctx context.Context, err error) string {
	return fmt.Sprintf("噩梦: %v", err)
}

func NewSleepTool(ch <-chan string) tool.BaseTool {
	t, err := utils.InferTool(
		"sleep",
		"睡上一会，阻塞等待被唤醒。上限 3h",
		SleepTool(ch),
	)
	if err != nil {
		log.Fatal(err)
	}
	return utils.WrapToolWithErrorHandler(t, SleepErrorHandler)
}
