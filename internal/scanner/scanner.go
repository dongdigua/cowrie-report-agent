package scanner

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	firstPort = 1
	lastPort  = 1024
	sshPort   = 22

	dialTimeout = time.Second
	readTimeout = time.Second
	concurrency = 16
)

type Scanner struct{}

type Args struct {
	IP string `json:"ip" jsonschema:"required,description=目标 IP 地址"`
}

type Result struct {
	Ports      []int
	SSHVersion string
}

func (a *Args) Validate() error {
	if a == nil {
		return errors.New("args is nil")
	}
	ip := strings.TrimSpace(a.IP)
	if ip == "" {
		return errors.New("ip is empty")
	}
	if _, err := netip.ParseAddr(ip); err != nil {
		return fmt.Errorf("ip %q is not an IPv4/IPv6 literal: %w", ip, err)
	}
	return nil
}

func (s *Scanner) Scan(args *Args, result *Result) error {
	log.Printf("%v\n", args)

	if err := args.Validate(); err != nil {
		return err
	}

	host := strings.TrimSpace(args.IP)

	ports := make([]int, 0, 16)
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, concurrency)
	)

	for port := firstPort; port <= lastPort; port++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(port int) {
			defer wg.Done()
			defer func() { <-sem }()

			conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), dialTimeout)
			if err != nil {
				return
			}
			defer conn.Close()

			version := ""
			if port == sshPort {
				version = readSSHBanner(conn)
			}

			mu.Lock()
			ports = append(ports, port)
			if version != "" {
				result.SSHVersion = version
			}
			mu.Unlock()
		}(port)
	}
	wg.Wait()

	sort.Ints(ports)
	result.Ports = ports
	return nil
}

func readSSHBanner(conn net.Conn) string {
	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		return ""
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	line = strings.TrimRight(line, "\r\n")
	if !strings.HasPrefix(line, "SSH-") {
		return ""
	}
	return line
}
