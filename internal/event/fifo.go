package event

import (
	"errors"
	"io"
	"log"
	"os"
)

const infifo = "in"

func Fifo(ch chan string) {
	buf := make([]byte, 4096)
	for {
		// A read-only open waits for a writer to connect.
		f, err := os.Open(infifo)
		if err != nil {
			log.Printf("open fifo: %v", err)
			return
		}

		info, err := f.Stat()
		if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
			f.Close()
			log.Printf("fifo: %s is not an accessible named pipe", infifo)
			return
		}

		for {
			n, readErr := f.Read(buf)
			if n > 0 {
				ch <- string(buf[:n])
			}
			if readErr != nil {
				err = readErr
				break
			}
		}
		f.Close()

		if !errors.Is(err, io.EOF) {
			log.Printf("read fifo: %v", err)
			return
		}
		// All writers disconnected; reopen to wait for the next writer.
	}
}
