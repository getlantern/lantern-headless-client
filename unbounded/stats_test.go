package unbounded

import (
	"net"
	"testing"
)

func TestStatsCountsDistinctPeersWithoutKeepingAddresses(t *testing.T) {
	s := newStats()
	a, b := net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")
	s.onConnectionChange(1, a)
	s.onConnectionChange(1, b)
	s.onConnectionChange(1, a)
	s.onConnectionChange(-1, nil)
	s.onDownstreamChunk(10)
	s.onUpstreamChunk(5)

	snap := s.snapshot()
	if snap.DistinctPeers != 2 || snap.Connects != 3 || snap.ActivePeers != 2 || snap.Total() != 15 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
	for key := range s.seen {
		if string(key[:]) == a.String() || string(key[:]) == b.String() {
			t.Fatal("raw peer address stored in stats")
		}
	}
	// A second process keys the same peer differently.
	if other := newStats(); other.peerKey(a) == s.peerKey(a) {
		t.Fatal("peer key is not salted per process")
	}
}
