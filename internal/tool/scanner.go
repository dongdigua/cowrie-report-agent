package tool

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"time"

	"cowrie-report-agent/internal/scanner"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func Scanner(ctx context.Context, in *scanner.Args) (*scanner.Result, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", os.Getenv("RPC_SERVER"))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := rpc.NewClient(conn)
	defer client.Close()

	stop := context.AfterFunc(ctx, func() { client.Close() })
	defer stop()

	var result scanner.Result
	if err := client.Call("Scanner.Scan", in, &result); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return &result, nil
}

func ScannerErrorHandler(ctx context.Context, err error) string {
	return fmt.Sprintf("端口探测失败：%v。", err)
}

func NewScannerTool() tool.BaseTool {
	t, err := utils.InferTool(
		"port_scanner",
		"探测指定 IP 的开放 TCP 端口（1-1024）并做服务指纹：22 端口返回 SSH 版本号，443 端口返回 TLS 信息",
		Scanner,
	)
	if err != nil {
		log.Fatal(err)
	}
	return utils.WrapToolWithErrorHandler(t, ScannerErrorHandler)
}
