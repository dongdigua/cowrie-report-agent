package scanner

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"runtime/debug"
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
	tlsPort   = 443

	dialTimeout = time.Second
	readTimeout = time.Second
	tlsTimeout  = time.Second
	concurrency = 16
)

type Scanner struct{}

type Args struct {
	IP string `json:"ip" jsonschema:"required,description=目标 IP 地址"`
}

type TLSInfo struct {
	Version     string   `json:"version,omitempty"`
	CipherSuite string   `json:"cipher_suite,omitempty"`
	ALPN        string   `json:"alpn,omitempty"`
	CommonName  string   `json:"common_name,omitempty"`
	DNSNames    []string `json:"dns_names,omitempty"`
	Issuer      string   `json:"issuer,omitempty"`
	NotAfter    string   `json:"not_after,omitempty"`
	// Error 记录握手失败原因；收到 TLS alert 说明该端口仍是 TLS 服务。
	Error string `json:"error,omitempty"`
}

type Result struct {
	Ports      []int    `json:"ports"`
	SSHVersion string   `json:"ssh_version,omitempty"`
	TLS        *TLSInfo `json:"tls,omitempty"`
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

func (s *Scanner) Scan(args *Args, result *Result) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("scanner: panic recovered: %v\n%s", r, debug.Stack())
			err = fmt.Errorf("scan panic: %v", r)
		}
	}()

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
			var info *TLSInfo
			if port == tlsPort {
				info = probeTLS(conn)
			}

			mu.Lock()
			defer mu.Unlock()
			ports = append(ports, port)
			if version != "" {
				result.SSHVersion = version
			}
			if info != nil {
				result.TLS = info
			}
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

func probeTLS(conn net.Conn) *TLSInfo {
	_ = conn.SetDeadline(time.Now().Add(tlsTimeout))
	tc := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		NextProtos:         []string{"h2", "http/1.1"},
	})
	if err := tc.Handshake(); err != nil {
		// 对端返回 TLS alert 时，Go 用 *net.OpError(Op="remote error") 包装，
		// 说明该端口确实是 TLS 服务，只是握手被拒（如要求 SNI 或客户端证书）。
		var opErr *net.OpError
		if errors.As(err, &opErr) && opErr.Op == "remote error" {
			return &TLSInfo{Error: err.Error()}
		}
		return nil
	}

	state := tc.ConnectionState()
	info := &TLSInfo{
		Version:     tls.VersionName(state.Version),
		CipherSuite: tls.CipherSuiteName(state.CipherSuite),
		ALPN:        state.NegotiatedProtocol,
	}
	if len(state.PeerCertificates) > 0 {
		leaf := state.PeerCertificates[0]
		info.CommonName = leaf.Subject.CommonName
		info.DNSNames = leaf.DNSNames
		info.Issuer = leaf.Issuer.String()
		info.NotAfter = leaf.NotAfter.UTC().Format(time.RFC3339)
	}
	return info
}
