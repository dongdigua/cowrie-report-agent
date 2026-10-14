package tool

import (
	"context"
	"log"
	"net/rpc"
	"os"

	"cowrie-report-agent/internal/scanner"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func Scanner(ctx context.Context, in *scanner.Args) (*scanner.Result, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	var result scanner.Result
	client, err := rpc.DialHTTP("tcp", os.Getenv("RPC_SERVER"))
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.Call("Scanner.Scan", in, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func NewScannerTool() tool.BaseTool {
	t, err := utils.InferTool(
		"port_scanner",
		"探测指定 IP 的开放 TCP 端口（1-1024），22 端口开放时返回 SSH 版本号",
		Scanner,
	)
	if err != nil {
		log.Fatal(err)
	}
	return t
}
