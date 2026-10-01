package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const gotifyHTTPTimeout = 10 * time.Second

type GotifySendInput struct {
	Title    string `json:"title" jsonschema:"required,description=通知标题"`
	Message  string `json:"message" jsonschema:"required,description=通知正文（支持 Markdown）"`
	Priority int    `json:"priority,omitempty" jsonschema:"description=通知优先级，数值越大越重要，默认 0"`
}

type GotifySendOutput struct {
	ID int64 `json:"id"`
	OK bool  `json:"ok"`
}

// GotifySend posts a message to the Gotify server.
func GotifySend(serverURL, appToken string) func(context.Context, *GotifySendInput) (*GotifySendOutput, error) {
	endpoint := strings.TrimRight(serverURL, "/") + "/message"
	client := &http.Client{Timeout: gotifyHTTPTimeout}

	return func(ctx context.Context, in *GotifySendInput) (*GotifySendOutput, error) {
		if strings.TrimSpace(in.Title) == "" && strings.TrimSpace(in.Message) == "" {
			return nil, fmt.Errorf("标题和正文不能同时为空")
		}

		form := url.Values{
			"title":    {in.Title},
			"message":  {in.Message},
			"priority": {fmt.Sprintf("%d", in.Priority)},
		}

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodPost,
			endpoint,
			strings.NewReader(form.Encode()),
		)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Gotify-Key", appToken)

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("gotify 返回状态码 %d：%s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		out := &GotifySendOutput{OK: true}

		// Gotify 成功时返回消息体，尽力解析出消息 ID，解析失败不影响发送结果。
		var sent struct {
			ID int64 `json:"id"`
		}
		if json.Unmarshal(body, &sent) == nil {
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
