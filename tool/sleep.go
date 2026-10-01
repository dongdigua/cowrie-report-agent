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
	Duration string `json:"duration" jsonschema:"required,description=睡眠时长，以 go duration string 表示`
}

type SleepOutput struct {
	Awake bool `json:"awake"`
	Now time.Time `json:"now"`
}

func SleepTool(ctx context.Context, in *SleepInput) (*SleepOutput, error) {
	dur, err := time.ParseDuration(in.Duration)
	if err != nil {
		return nil, err
	}

	if dur > 3 * time.Hour {
		return nil, fmt.Errorf("duration too long (need <= 3h)")
	}

	timer := time.NewTimer(dur)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}

	now := time.Now().In(beijing)
	return &SleepOutput{Awake: true, Now: now}, nil
}

func SleepErrorHandler(ctx context.Context, err error) string {
	return fmt.Sprintf("噩梦: %v", err)
}

func NewSleepTool() tool.BaseTool {
	t, err := utils.InferTool(
		"sleep",
		"睡上一会，阻塞等待被唤醒。",
		SleepTool,
	)
	if err != nil {
		log.Fatal(err)
	}
	return utils.WrapToolWithErrorHandler(t, SleepErrorHandler)
}
