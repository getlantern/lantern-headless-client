// Package unbounded runs this host as an Unbounded volunteer peer: it answers
// censored users' WebRTC offers brokered by Freddie and relays their traffic
// to a Lantern egress server over WebSocket. It wraps the native widget role
// of github.com/getlantern/broflake/clientcore with the limits a long-running
// service needs: concurrent peers, byte rate, and a data cap per period.
package unbounded

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// Production swarm, from unbounded's ui/.env.production.example. The native
// WebSocket dialer needs the egress port spelled out.
const (
	DefaultFreddie = "https://freddie.iantem.io"
	DefaultEgress  = "wss://unbounded.iantem.io:443"
)

// Config is everything Run needs. Zero values mean "no limit" where a limit
// is optional and the broflake default elsewhere.
type Config struct {
	// Freddie is the discovery (signaling) server.
	Freddie string
	// Egress is the WebSocket URL of the egress server this peer relays to.
	Egress string
	// MaxPeers bounds how many censored peers are served at once; it sizes
	// both of the widget's worker tables.
	MaxPeers int
	// BandwidthLimit caps the relayed byte rate in each direction. 0 is unlimited.
	BandwidthLimit Rate
	// DataCap pauses serving once this much has been relayed in the current
	// period. The zero value means no cap.
	DataCap DataCap
	// TargetCountries lists ISO-3166-1 alpha-2 countries whose users this peer
	// prefers to serve. Empty serves everyone.
	TargetCountries []string
	// Tag labels this peer in Freddie's and netstated's view. Optional.
	Tag string
	// Netstated is the network-state reporting endpoint. Optional.
	Netstated string
	// NATFailTimeout bounds how long a WebRTC attempt may spend in ICE.
	NATFailTimeout time.Duration
	// StatsInterval is how often the stats line is logged. 0 disables it.
	StatsInterval time.Duration
	// SkipNATCheck skips the startup NAT mapping probe.
	SkipNATCheck bool
	// PprofAddr, when set, serves net/http/pprof there.
	PprofAddr string
	// DataPath is where usage state is persisted.
	DataPath string
	// Version is reported in the startup log line.
	Version string
}

// Rate is a byte rate in bytes per second.
type Rate uint64

// ParseRate reads a rate such as "5mbit", "20Mbps", "500KB/s", "1.5MiB/s" or
// "2MB". The unit letter decides bits versus bytes the way network tooling
// does: a lowercase b, "bit" or "bps" means bits, an uppercase B or "byte"
// means bytes. k, M and G multiply by 1000; Ki, Mi and Gi by 1024. "0" and
// "" mean unlimited.
func ParseRate(s string) (Rate, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}
	m := rateRe.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("unrecognized rate %q (examples: 5mbit, 500KB/s, 1.5MiB/s)", s)
	}
	value, err := strconv.ParseFloat(m[1], 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("unrecognized rate %q", s)
	}
	mult := 1.0
	base := 1000.0
	if m[3] != "" {
		base = 1024
	}
	switch strings.ToLower(m[2]) {
	case "k":
		mult = base
	case "m":
		mult = base * base
	case "g":
		mult = base * base * base
	}
	bits := false
	switch m[4] {
	case "bit", "bits", "bps", "b":
		bits = true
	}
	bytesPerSec := value * mult
	if bits {
		bytesPerSec /= 8
	}
	if bytesPerSec < 1 {
		return 0, fmt.Errorf("rate %q is below 1 byte per second", s)
	}
	return Rate(bytesPerSec), nil
}

// rateRe: number, optional k/m/g prefix, optional i for binary, unit, optional
// per-second suffix. The unit group is case-sensitive on purpose (b vs B).
var rateRe = regexp.MustCompile(`^([0-9]*\.?[0-9]+)\s*([kKmMgG]?)(i?)(bits?|bps|b|bytes?|B)(?:/s|ps)?$`)

func (r Rate) String() string {
	if r == 0 {
		return "unlimited"
	}
	return humanize.Bytes(uint64(r)) + "/s"
}

// Period is the window a DataCap applies to.
type Period int

const (
	PeriodNone Period = iota
	PeriodDay
	PeriodWeek
	PeriodMonth
)

func (p Period) String() string {
	switch p {
	case PeriodDay:
		return "day"
	case PeriodWeek:
		return "week"
	case PeriodMonth:
		return "month"
	default:
		return "none"
	}
}

// DataCap is a byte budget per period. Bytes == 0 means no cap.
type DataCap struct {
	Bytes  uint64
	Period Period
}

// ParseDataCap reads a cap such as "50GB/day", "200GiB/week" or "1TB/month".
// Without a period the cap is per day. "" and "0" mean no cap.
func ParseDataCap(s string) (DataCap, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return DataCap{}, nil
	}
	size, period := s, "day"
	if i := strings.LastIndex(s, "/"); i >= 0 {
		size, period = strings.TrimSpace(s[:i]), strings.ToLower(strings.TrimSpace(s[i+1:]))
	}
	bytes, err := humanize.ParseBytes(size)
	if err != nil {
		return DataCap{}, fmt.Errorf("unrecognized data cap %q: %w", s, err)
	}
	if bytes == 0 {
		return DataCap{}, nil
	}
	var p Period
	switch period {
	case "day", "daily", "d":
		p = PeriodDay
	case "week", "weekly", "w":
		p = PeriodWeek
	case "month", "monthly", "mo", "m":
		p = PeriodMonth
	default:
		return DataCap{}, fmt.Errorf("unrecognized data cap period %q in %q (day, week or month)", period, s)
	}
	return DataCap{Bytes: bytes, Period: p}, nil
}

func (c DataCap) String() string {
	if c.Bytes == 0 {
		return "none"
	}
	return humanize.Bytes(c.Bytes) + "/" + c.Period.String()
}

// Validate rejects a Config Run could not act on.
func (c Config) Validate() error {
	if c.Freddie == "" {
		return errors.New("discovery server (freddie) is required")
	}
	if c.Egress == "" {
		return errors.New("egress address is required")
	}
	if c.MaxPeers < 1 {
		return errors.New("max peers must be at least 1")
	}
	if c.NATFailTimeout <= 0 {
		return errors.New("NAT fail timeout must be positive")
	}
	return nil
}
