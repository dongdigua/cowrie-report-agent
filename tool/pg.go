package tool

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type PgQueryInput struct {
	SQL string `json:"sql" jsonschema:"required,description=要执行的 SQL 语句"`
}

type PgQueryOutput struct {
	Fields []string `json:"fields"`
	Rows   [][]any  `json:"rows"`
	Trunc  bool     `json:"hard_truncated"`
}

func PgQuery(pool *pgxpool.Pool) func(context.Context, *PgQueryInput) (*PgQueryOutput, error) {
	return func(ctx context.Context, in *PgQueryInput) (*PgQueryOutput, error) {
		rows, err := pool.Query(ctx, in.SQL)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		fieldDescs := rows.FieldDescriptions()
		cols := make([]string, len(fieldDescs))
		for i, fd := range fieldDescs {
			cols[i] = fd.Name
		}

		var result [][]any
		trunc := false

		for rows.Next() {
			if len(result) >= 101 {
				trunc = true
				break
			}
			vals, err := rows.Values()
			if err != nil {
				return nil, err
			}
			result = append(result, vals)
		}

		if err := rows.Err(); err != nil {
			return nil, err
		}

		return &PgQueryOutput{Fields: cols, Rows: result, Trunc: trunc}, nil
	}
}

func PgErrorHandler(ctx context.Context, err error) string {
	return fmt.Sprintf("数据库查询失败：%v。", err)
}

func NewPgQueryTool(pool *pgxpool.Pool) tool.BaseTool {
	t, err := utils.InferTool(
		"pg_query",
		"连接 Postgres 数据库进行只读查询",
		PgQuery(pool),
	)
	if err != nil {
		log.Fatal(err)
	}
	return utils.WrapToolWithErrorHandler(t, PgErrorHandler)
}
