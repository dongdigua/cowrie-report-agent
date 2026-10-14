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
	Finished bool `json:"finished"`
}

func FinishSession(ctx context.Context, in *FinishSessionInput) (*FinishSessionOutput, error) {
	return &FinishSessionOutput{Finished: true}, nil
}

func NewFinishSessionTool() tool.BaseTool {
	t, err := utils.InferTool(
		"finish_session",
		"结束当前 session：调用后 TurnLoop 将在本轮结束后退出。当研判任务已完成、无需继续运行时使用。",
		FinishSession,
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
