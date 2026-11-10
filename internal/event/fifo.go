package event

import (
	"io"
	"os"
	"log"
	"errors"
)

const infifo = "in"

func Fifo(ch chan string) {
	// O_RDWR:
	// - open 不阻塞（不需要事先有写端）
	// - 自己持有一个写端 -> 外部写端关闭时也不会读到 EO
	f, err := os.OpenFile(infifo, os.O_RDWR, 0)
	if err != nil {
		log.Printf("fifo not found, ignoring")
		return
	}

	defer f.Close()

	buf := make([]byte, 4096)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			ch <- string(buf[:n])
		}
		switch {
		case err == nil:
			continue
		case errors.Is(err, os.ErrClosed):
			return // signal
		case errors.Is(err, io.EOF):
			// O_RDWR 下理论上不会发生；保险起见继续
			continue
		default:
			log.Printf("read: %v", err)
			return
		}
	}
}
