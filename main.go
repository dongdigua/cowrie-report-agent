package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	agenttool "cowrie-report-agent/tool"

	"github.com/joho/godotenv"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-examples/adk/common/prints"
	"github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	ctx := context.Background()

	dbpool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer dbpool.Close()

	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:          os.Getenv("OPENAI_API_KEY"),
		Model:           os.Getenv("OPENAI_MODEL"),
		BaseURL:         os.Getenv("OPENAI_BASE_URL"),
		ReasoningEffort: openai.ReasoningEffortLevelMedium,
	})
	if err != nil {
		log.Fatal(err)
	}

	sleepch := make(chan string)

	tools := []tool.BaseTool{
		agenttool.NewCurrentTimeTool(),
		agenttool.NewSleepTool(sleepch),
		agenttool.NewPgQueryTool(dbpool),
		agenttool.NewGotifySendTool(
			os.Getenv("GOTIFY_URL"),
			os.Getenv("GOTIFY_TOKEN"),
		),
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "cowrie_agent",
		Description: "A friendly greeting assistant",
		// Instruction: "你是一位安全审计专家，需要从 cowrie 数据库中总结近期威胁情报并对比历史情报，研判高危事件。查询内容时一定要加 LIMIT，不要全量查询。最后生成用于上报给网信部门的简要报告。若有正在进行中的攻击，根据攻击频率自行决定是否产生报告和下一次查询数据库的时间",
		Instruction: "你是一位安全审计专家，需要从 cowrie 数据库中(查询内容时要加 LIMIT，不要全量查询)总结近期威胁情报并对比历史情报，研判高危事件。最后生成用于上报给网信部门的简要报告。",
		Model:       model,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	cfg := adk.TurnLoopConfig[string, *schema.Message]{
		// GenInput：接收缓冲区所有项目，决定本轮消费哪些
		GenInput: func(ctx context.Context, loop *adk.TurnLoop[string, *schema.Message], items []string) (*adk.GenInputResult[string, *schema.Message], error) {
			return &adk.GenInputResult[string, *schema.Message]{
				Input:    &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage(strings.Join(items, "\n"))}},
				Consumed: items,
			}, nil
		},

		// PrepareAgent：根据本轮消费项构建 Agent
		PrepareAgent: func(ctx context.Context, loop *adk.TurnLoop[string, *schema.Message], consumed []string) (adk.Agent, error) {
			return agent, nil
		},

		// OnAgentEvents：接收 Agent 的事件流，负责渲染输出和持久化中间消息
		OnAgentEvents: func(_ context.Context, _ *adk.TurnContext[string, *schema.Message], events *adk.AsyncIterator[*adk.TypedAgentEvent[*schema.Message]]) error {
			for {
				event, ok := events.Next()
				if !ok {
					return nil
				}

				if event.Err != nil {
					log.Printf("错误: %v", event.Err)
					return nil
				}

				prints.Event(event)
			}
		},
	}

	loop := adk.NewTurnLoop(cfg)

	loop.Push(os.Args[1])
	loop.Run(ctx) // 非阻塞
	loop.Stop(adk.UntilIdleFor(8 * time.Hour))

	result := loop.Wait() // 阻塞至退出

	if result.ExitReason != nil {
		log.Print(result.ExitReason)
	}
}
