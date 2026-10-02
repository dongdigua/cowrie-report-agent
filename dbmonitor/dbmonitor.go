package dbmonitor

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const triggerThreshold = 100

func DbMonitor(ctx context.Context, pool *pgxpool.Pool, ch chan string) {
	lastVal := 0
	for {
		var curVal int
		err := pool.QueryRow(ctx, "select last_value from auth_id_seq").Scan(&curVal)
		if err != nil {
			log.Printf("dbmonitor: QueryRow failed: %v", err)
		}

		if lastVal != 0 && curVal > lastVal+triggerThreshold {
			msg := fmt.Sprintf("过去一小时内，auth 表新增了 %d 行", curVal-lastVal)
			select {
			case ch <- msg:
			default:
			}
		}
		lastVal = curVal
		time.Sleep(time.Hour)
	}
}
