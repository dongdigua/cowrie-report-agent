package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/template"
	"time"

	"cowrie-report-agent/internal/dbmonitor"
	agenttool "cowrie-report-agent/internal/tool"

	"github.com/joho/godotenv"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-examples/adk/common/prints"
	"github.com/cloudwego/eino-ext/components/model/openai"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed prompt
var prompt string

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	f, err := os.Open("prompt")
	defer f.Close()
	if err == nil {
		content, err := io.ReadAll(f)
		if err == nil {
			prompt = string(content)
			fmt.Println("local prompt override")
		}
	}

	tpl, err := template.New("prompt").Parse(prompt)
	if err != nil {
		log.Fatalf("Error parsing template %v\n", tpl)
	}

	var buf bytes.Buffer
	tpl.Execute(&buf, time.Now().In(agenttool.LoadLocation()).Format("2006-01-02 15:04:05"))
	prompt = buf.String()
	// fmt.Println(prompt)

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
		ReasoningEffort: openai.ReasoningEffortLevel(os.Getenv("OPENAI_REASONING_LEVEL")),
	})
	if err != nil {
		log.Fatal(err)
	}

	datach := make(chan string, 1)

	go dbmonitor.DbMonitor(ctx, dbpool, datach)

	tools := []tool.BaseTool{
		agenttool.NewCurrentTimeTool(),
		agenttool.NewFinishSessionTool(),
		agenttool.NewSleepTool(datach),
		agenttool.NewPgQueryTool(dbpool),
		agenttool.NewGotifySendTool(
			os.Getenv("GOTIFY_URL"),
			os.Getenv("GOTIFY_TOKEN"),
		),
	}
	if os.Getenv("RPC_SERVER") == "" {
		log.Println("no RPC_SERVER found! port scanner is disabled")
	} else {
		tools = append(tools, agenttool.NewScannerTool())
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "cowrie_agent",
		Description:   "Cowrie 威胁情报研判与上报助手",
		Instruction:   prompt,
		Model:         model,
		MaxIterations: 256,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
			ReturnDirectly: map[string]bool{"finish_session": true},
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

	if len(os.Args) > 1 { // one-shot
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
			prints.Event(event)
		}
		return
	}

	for {
		dbtrigger := <-datach

		loop := adk.NewTurnLoop(cfg)

		loop.Push(fmt.Sprintf("你醒啦：%s", dbtrigger))
		loop.Run(ctx) // 非阻塞
		loop.Stop(adk.UntilIdleFor(8 * time.Hour))
		result := loop.Wait() // 阻塞至退出

		if result.ExitReason != nil {
			log.Print(result.ExitReason)
		}
	}
}
