package dbmonitor

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const triggerThreshold = 100

func DbMonitor(ctx context.Context, pool *pgxpool.Pool, ch chan string) {
	lastVal := 0
	for {
		var curVal int
		pool.QueryRow(ctx, "select last_value from auth_id_seq").Scan(&curVal)
		if lastVal != 0 && curVal > lastVal + triggerThreshold {
			ch <- fmt.Sprintf("过去一小时内，auth 表新增了 %d 行", curVal-lastVal)
		}
		lastVal = curVal
		time.Sleep(time.Hour)
	}
}
