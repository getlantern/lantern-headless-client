package unbounded

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// stats are the lifetime counters a volunteer wants to see: how much it has
// relayed, for how many peers, in each direction. They are fed by broflake's
// connection and chunk callbacks. The chunk callbacks run on the data path,
// so they are atomic adds and nothing else. Directions are from this peer's
// point of view as a relay: "to peers" is what it downloads from the egress
// for a consumer, "from peers" is what it uploads for them.
//
// Peers are censored users, so their addresses are not kept: distinct peers
// are counted by a hash salted per process, which identifies nobody once the
// process exits.
type stats struct {
	bytesToPeers   atomic.Uint64
	bytesFromPeers atomic.Uint64
	connects       atomic.Uint64
	active         atomic.Int64
	// lastActivityNanos is when a callback last fired, for draining on stop.
	lastActivityNanos atomic.Int64

	salt [16]byte
	mu   sync.Mutex
	seen map[[sha256.Size]byte]struct{}
}

func newStats() *stats {
	s := &stats{seen: make(map[[sha256.Size]byte]struct{})}
	_, _ = rand.Read(s.salt[:])
	return s
}

func (s *stats) peerKey(addr net.IP) [sha256.Size]byte {
	h := sha256.New()
	h.Write(s.salt[:])
	h.Write(addr)
	var key [sha256.Size]byte
	copy(key[:], h.Sum(nil))
	return key
}

func (s *stats) touch() {
	s.lastActivityNanos.Store(time.Now().UnixNano())
}

// lastActivity is when a callback last fired; the zero time if none has.
func (s *stats) lastActivity() time.Time {
	n := s.lastActivityNanos.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

func (s *stats) onConnectionChange(state int, addr net.IP) {
	s.touch()
	switch {
	case state > 0:
		s.connects.Add(1)
		s.active.Add(1)
		if addr != nil {
			key := s.peerKey(addr)
			s.mu.Lock()
			s.seen[key] = struct{}{}
			s.mu.Unlock()
		}
	case state < 0:
		s.active.Add(-1)
	}
}

func (s *stats) onDownstreamChunk(size int) {
	s.touch()
	if size > 0 {
		s.bytesToPeers.Add(uint64(size))
	}
}

func (s *stats) onUpstreamChunk(size int) {
	s.touch()
	if size > 0 {
		s.bytesFromPeers.Add(uint64(size))
	}
}

// Snapshot is a copy of the counters at one moment.
type Snapshot struct {
	BytesToPeers   uint64
	BytesFromPeers uint64
	Connects       uint64
	ActivePeers    int64
	DistinctPeers  int
}

func (s *stats) snapshot() Snapshot {
	active := s.active.Load()
	if active < 0 {
		active = 0
	}
	s.mu.Lock()
	distinct := len(s.seen)
	s.mu.Unlock()
	return Snapshot{
		BytesToPeers:   s.bytesToPeers.Load(),
		BytesFromPeers: s.bytesFromPeers.Load(),
		Connects:       s.connects.Load(),
		ActivePeers:    active,
		DistinctPeers:  distinct,
	}
}

// Total is the bytes relayed in both directions.
func (s Snapshot) Total() uint64 {
	return s.BytesToPeers + s.BytesFromPeers
}

// logAttrs renders the snapshot as slog key/value pairs.
func (s Snapshot) logAttrs() []any {
	return []any{
		"active_peers", s.ActivePeers,
		"distinct_peers", s.DistinctPeers,
		"connections", s.Connects,
		"to_peers", humanBytes(s.BytesToPeers),
		"from_peers", humanBytes(s.BytesFromPeers),
		"total", humanBytes(s.Total()),
	}
}

// humanBytes renders a byte count as a human-readable string (e.g. "1.5 MiB").
func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
