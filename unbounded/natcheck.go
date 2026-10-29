package unbounded

import (
	"context"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pion/stun/v3"
)

// NATMapping is what the startup probe learned about this host's NAT. A
// volunteer can only serve peers over STUN (Unbounded has no TURN), so
// whether the NAT keeps one public mapping across destinations is the single
// biggest predictor of whether it can hole-punch at all.
type NATMapping int

const (
	NATMappingUnknown NATMapping = iota
	// NATMappingEndpointIndependent: a cone NAT. STUN hole-punching works.
	NATMappingEndpointIndependent
	// NATMappingEndpointDependent: a symmetric NAT. The public port changes per
	// destination, so STUN-only P2P fails for most peers.
	NATMappingEndpointDependent
)

func (m NATMapping) String() string {
	switch m {
	case NATMappingEndpointIndependent:
		return "endpoint-independent"
	case NATMappingEndpointDependent:
		return "endpoint-dependent"
	default:
		return "unknown"
	}
}

// NATCheckResult reports what the mapping probe observed.
type NATCheckResult struct {
	Mapping     NATMapping
	PublicAddrs []string // distinct reflexive addresses observed, "ip:port"
	Samples     int      // number of STUN servers that answered
}

// CheckNATMapping is an RFC 5780-style mapping-behavior probe. It sends STUN
// Binding requests from one local socket to several servers and compares the
// reflexive addresses they report: all identical means endpoint-independent,
// any difference means endpoint-dependent. This is independent of pion's ICE
// gathering, which uses a socket per STUN server and so says nothing about
// NAT type. Servers are "host:port" or "stun:host:port". perServerTimeout
// bounds each request; ctx bounds the whole probe.
func CheckNATMapping(ctx context.Context, stunServers []string, perServerTimeout time.Duration) NATCheckResult {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return NATCheckResult{}
	}
	defer conn.Close()

	var addrs []string
	samples := 0
	for _, s := range stunServers {
		if ctx.Err() != nil {
			break
		}
		ip, port, err := stunMappedAddr(ctx, conn, s, perServerTimeout)
		if err != nil {
			continue
		}
		samples++
		addrs = append(addrs, net.JoinHostPort(ip.String(), strconv.Itoa(port)))
	}
	return classifyMapping(addrs, samples)
}

// classifyMapping turns the observed reflexive addresses into a verdict.
// Fewer than two answers is not enough to say anything.
func classifyMapping(observed []string, samples int) NATCheckResult {
	seen := map[string]struct{}{}
	var distinct []string
	for _, a := range observed {
		if _, ok := seen[a]; !ok {
			seen[a] = struct{}{}
			distinct = append(distinct, a)
		}
	}
	res := NATCheckResult{PublicAddrs: distinct, Samples: samples}
	switch {
	case samples < 2:
		res.Mapping = NATMappingUnknown
	case len(distinct) == 1:
		res.Mapping = NATMappingEndpointIndependent
	default:
		res.Mapping = NATMappingEndpointDependent
	}
	return res
}

// stunMappedAddr sends one Binding request to server on conn and returns the
// reflexive address the server reports.
func stunMappedAddr(ctx context.Context, conn *net.UDPConn, server string, timeout time.Duration) (net.IP, int, error) {
	server = strings.TrimPrefix(strings.TrimSpace(server), "stun:")
	saddr, err := net.ResolveUDPAddr("udp4", server)
	if err != nil {
		return nil, 0, err
	}
	msg := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	if _, err := conn.WriteToUDP(msg.Raw, saddr); err != nil {
		return nil, 0, err
	}

	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetReadDeadline(deadline)

	buf := make([]byte, 1500)
	// Loop past stray datagrams from other servers until one decodes as a
	// valid STUN success carrying an XOR-MAPPED-ADDRESS, or the deadline passes.
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return nil, 0, err
		}
		resp := &stun.Message{Raw: append([]byte{}, buf[:n]...)}
		if resp.Decode() != nil {
			continue
		}
		var xor stun.XORMappedAddress
		if xor.GetFrom(resp) != nil {
			continue
		}
		return xor.IP, xor.Port, nil
	}
}

// logNATCheck probes the NAT once at startup and logs the verdict, so an
// operator behind a symmetric NAT learns up front that this host cannot help
// instead of inferring it from failed attempts.
func logNATCheck(ctx context.Context, stunBatch func(size uint32) ([]string, error)) {
	servers, err := stunBatch(8)
	if err != nil || len(servers) < 2 {
		slog.Warn("NAT mapping self-check skipped", "error", err, "stun_servers", len(servers))
		return
	}
	if len(servers) > 5 {
		servers = servers[:5]
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	res := CheckNATMapping(ctx, servers, 3*time.Second)

	switch res.Mapping {
	case NATMappingEndpointIndependent:
		slog.Info("NAT mapping self-check",
			"result", res.Mapping.String(),
			"public", res.PublicAddrs,
			"samples", res.Samples,
			"detail", "stable public mapping across STUN servers; STUN hole-punching is viable from this host",
		)
	case NATMappingEndpointDependent:
		slog.Warn("NAT mapping self-check",
			"result", res.Mapping.String(),
			"public", res.PublicAddrs,
			"samples", res.Samples,
			"detail", "SYMMETRIC NAT: the public port varies by destination, so STUN-only P2P (Unbounded has no TURN) will fail for most peers; put this host on a public IP or a cone NAT to serve reliably",
		)
	default:
		slog.Warn("NAT mapping self-check inconclusive",
			"samples", res.Samples,
			"detail", "not enough STUN responses to determine NAT mapping behavior",
		)
	}
}
