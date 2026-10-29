package unbounded

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/pion/stun/v3"
)

func TestClassifyMapping(t *testing.T) {
	cases := []struct {
		name     string
		observed []string
		samples  int
		want     NATMapping
	}{
		{"one answer", []string{"1.2.3.4:1000"}, 1, NATMappingUnknown},
		{"no answers", nil, 0, NATMappingUnknown},
		{"stable", []string{"1.2.3.4:1000", "1.2.3.4:1000", "1.2.3.4:1000"}, 3, NATMappingEndpointIndependent},
		{"port varies", []string{"1.2.3.4:1000", "1.2.3.4:1001"}, 2, NATMappingEndpointDependent},
	}
	for _, c := range cases {
		if got := classifyMapping(c.observed, c.samples).Mapping; got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// A local STUN responder that reports the sender's address: on loopback every
// server sees the same mapping, so the verdict is endpoint-independent.
func TestCheckNATMappingAgainstLocalResponders(t *testing.T) {
	var servers []string
	for i := 0; i < 2; i++ {
		srv, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		defer srv.Close()
		servers = append(servers, "stun:"+srv.LocalAddr().String())
		go func(srv *net.UDPConn) {
			buf := make([]byte, 1500)
			for {
				n, from, err := srv.ReadFromUDP(buf)
				if err != nil {
					return
				}
				req := &stun.Message{Raw: append([]byte{}, buf[:n]...)}
				if req.Decode() != nil {
					continue
				}
				resp := stun.MustBuild(req, stun.BindingSuccess,
					&stun.XORMappedAddress{IP: from.IP, Port: from.Port}, stun.Fingerprint)
				_, _ = srv.WriteToUDP(resp.Raw, from)
			}
		}(srv)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res := CheckNATMapping(ctx, servers, time.Second)
	if res.Samples != 2 {
		t.Fatalf("got %d samples, want 2", res.Samples)
	}
	if res.Mapping != NATMappingEndpointIndependent {
		t.Fatalf("got %v (%v), want endpoint-independent", res.Mapping, res.PublicAddrs)
	}
}
