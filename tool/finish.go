package tool

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type FinishSessionInput struct {
	Reason string `json:"reason,omitempty" jsonschema:"description=结束本次 session 的原因，会作为 loop 的停止原因"`
}

type FinishSessionOutput struct {
	Finished bool   `json:"finished"`
	Reason   string `json:"reason,omitempty"`
}

func FinishSession(ch chan<- string) func(context.Context, *FinishSessionInput) (*FinishSessionOutput, error) {
	return func(ctx context.Context, in *FinishSessionInput) (*FinishSessionOutput, error) {
		select {
		case ch <- in.Reason:
		default: // 无人监听时丢弃，避免阻塞当前轮次
		}
		return &FinishSessionOutput{Finished: true, Reason: in.Reason}, nil
	}
}

func NewFinishSessionTool(ch chan<- string) tool.BaseTool {
	t, err := utils.InferTool(
		"finish_session",
		"结束当前 session：调用后 TurnLoop 将在本轮结束后退出。当研判任务已完成、无需继续运行时使用。",
		FinishSession(ch),
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
