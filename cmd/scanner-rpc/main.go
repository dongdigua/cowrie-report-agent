package main

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"

	"cowrie-report-agent/internal/scanner"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	srv := new(scanner.Scanner)
	if err := rpc.Register(srv); err != nil {
		log.Fatal("register error:", err)
	}
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", os.Getenv("RPC_LISTEN"))
	if err != nil {
		log.Fatal("listen error:", err)
	}
	log.Fatal(http.Serve(l, nil))
}
