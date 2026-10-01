package main

import (
	"context"
	"log"
	"os"

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
		log.Fatal(os.Stderr, "Unable to create connection pool: %v\n", err)
	}
	defer dbpool.Close()

	model, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		Model:   os.Getenv("OPENAI_MODEL"),
		BaseURL: os.Getenv("OPENAI_BASE_URL"),
		ByAzure: func() bool {
			return os.Getenv("OPENAI_BY_AZURE") == "true"
		}(),
	})
	if err != nil {
		log.Fatal(err)
	}

	tools := []tool.BaseTool{
		agenttool.NewCurrentTimeTool(),
		agenttool.NewPgQueryTool(dbpool),
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "cowrie_agent",
		Description: "A friendly greeting assistant",
		// Instruction: "你是一位安全审计专家，需要从 cowrie 数据库中总结近期威胁情报并对比历史情报，研判高危事件。查询内容时一定要加 LIMIT，不要全量查询。最后生成用于上报给网信部门的简要报告。若有正在进行中的攻击，根据攻击频率自行决定是否产生报告和下一次查询数据库的时间",
		Instruction: "你是一位安全审计专家，需要从 cowrie 数据库中总结近期威胁情报并对比历史情报，研判高危事件。查询内容时一定要加 LIMIT，不要全量查询。最后生成用于上报给网信部门的简要报告。",
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

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
	})
	input := []adk.Message{
		schema.UserMessage(os.Args[1]),
	}

	events := runner.Run(ctx, input)
	for {
		event, ok := events.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			log.Printf("错误: %v", event.Err)
			break
		}

		prints.Event(event)
	}
}
