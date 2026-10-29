package unbounded

import (
	"net"
	"testing"
	"time"
)

// Sending through a limited socket must take about the time the limit
// implies: 512 KiB at 1 MiB/s, less the burst admitted up front.
func TestLimitedNetShapesWrites(t *testing.T) {
	const limit = Rate(1 << 20)
	ln, err := NewLimitedNet(limit)
	if err != nil {
		t.Fatal(err)
	}
	sender, err := ln.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	sink, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	const packet = 1024
	const packets = 512
	buf := make([]byte, packet)
	start := time.Now()
	for i := 0; i < packets; i++ {
		if _, err := sender.WriteTo(buf, sink.LocalAddr()); err != nil {
			t.Fatal(err)
		}
	}
	elapsed := time.Since(start)

	// The burst is admitted at once; the rest is paced at the limit.
	burst := newLimiter(limit).Burst()
	want := time.Duration(float64(packets*packet-burst) / float64(limit) * float64(time.Second))
	if elapsed < want*8/10 {
		t.Fatalf("sent %d bytes in %v; limit should have taken at least %v", packets*packet, elapsed, want)
	}
	if elapsed > want*3 {
		t.Fatalf("sent %d bytes in %v; far slower than the limit's %v", packets*packet, elapsed, want)
	}
}

func TestLimitedNetOversizedWriteIsNotBlocked(t *testing.T) {
	ln, err := NewLimitedNet(1000)
	if err != nil {
		t.Fatal(err)
	}
	sender, err := ln.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	sink, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()

	done := make(chan error, 1)
	go func() {
		_, err := sender.WriteTo(make([]byte, minBurst+1), sink.LocalAddr())
		done <- err
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a datagram larger than the burst must not block forever")
	}
}
