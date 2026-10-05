package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const gotifyHTTPTimeout = 10 * time.Second

type GotifySendInput struct {
	Title    string `json:"title" jsonschema:"required,description=通知标题"`
	Message  string `json:"message" jsonschema:"required,description=通知正文（支持 Markdown，不支持表格）"`
	Priority int    `json:"priority,omitempty" jsonschema:"description=通知优先级，数值越大越重要，默认 0"`
}

type GotifySendOutput struct {
	ID int64 `json:"id"`
	OK bool  `json:"ok"`
}

type gotifyMessage struct {
	Message  string         `json:"message"`
	Title    string         `json:"title,omitempty"`
	Priority int            `json:"priority"`
	Extras   map[string]any `json:"extras,omitempty"`
}

// GotifySend posts a message to the Gotify server.
func GotifySend(serverURL, appToken string) func(context.Context, *GotifySendInput) (*GotifySendOutput, error) {
	endpoint := strings.TrimRight(serverURL, "/") + "/message"
	client := &http.Client{Timeout: gotifyHTTPTimeout}

	return func(ctx context.Context, in *GotifySendInput) (*GotifySendOutput, error) {
		if strings.TrimSpace(in.Title) == "" && strings.TrimSpace(in.Message) == "" {
			return nil, fmt.Errorf("标题和正文不能同时为空")
		}

		// extras 只在 application/json 请求中生效，用于让客户端按 Markdown 渲染正文。
		payload := gotifyMessage{
			Message:  in.Message,
			Title:    in.Title,
			Priority: in.Priority,
			Extras: map[string]any{
				"client::display": map[string]any{
					"contentType": "text/markdown",
				},
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			endpoint,
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gotify-Key", appToken)

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("gotify 返回状态码 %d：%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}

		out := &GotifySendOutput{OK: true}

		// Gotify 成功时返回消息体，尽力解析出消息 ID，解析失败不影响发送结果。
		var sent struct {
			ID int64 `json:"id"`
		}
		if json.Unmarshal(respBody, &sent) == nil {
			out.ID = sent.ID
		}

		return out, nil
	}
}

func GotifyErrorHandler(ctx context.Context, err error) string {
	return fmt.Sprintf("发送 Gotify 通知失败：%v。", err)
}

// NewGotifySendTool constructs the "send gotify notification" tool.
func NewGotifySendTool(serverURL, appToken string) tool.BaseTool {
	t, err := utils.InferTool(
		"gotify",
		"通过 Gotify 推送通知。",
		GotifySend(serverURL, appToken),
	)
	if err != nil {
		log.Fatal(err)
	}
	return utils.WrapToolWithErrorHandler(t, GotifyErrorHandler)
}
