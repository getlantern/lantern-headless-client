package unbounded

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"time"

	"github.com/getlantern/broflake/clientcore"
	"github.com/pion/transport/v5"
)

// housekeepingInterval is how often Run persists the data-cap count and
// checks for a period boundary while serving.
const housekeepingInterval = time.Minute

// Run serves as an Unbounded volunteer until ctx is done. A data cap pauses
// serving until the next period and then starts a fresh engine, since a
// stopped broflake engine cannot be restarted.
func Run(ctx context.Context, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	st := newStats()

	var cap *capTracker
	if cfg.DataCap.Bytes > 0 {
		var err error
		cap, err = newCapTracker(cfg.DataCap, cfg.DataPath, time.Now)
		if err != nil {
			return fmt.Errorf("data cap: %w", err)
		}
	}

	var rtcNet transport.Net
	if cfg.BandwidthLimit > 0 {
		var err error
		rtcNet, err = NewLimitedNet(cfg.BandwidthLimit)
		if err != nil {
			return fmt.Errorf("bandwidth limit: %w", err)
		}
	}

	slog.Info("Unbounded volunteer peer starting",
		"version", cfg.Version,
		"discovery", cfg.Freddie,
		"egress", cfg.Egress,
		"max_peers", cfg.MaxPeers,
		"bandwidth_limit", cfg.BandwidthLimit.String(),
		"data_cap", cfg.DataCap.String(),
		"target_countries", orNone(strings.Join(cfg.TargetCountries, ",")),
		"nat_fail_timeout", cfg.NATFailTimeout,
		"tag", orNone(cfg.Tag),
		"netstated", orNone(cfg.Netstated),
	)
	if cap != nil && cap.Used() > 0 {
		slog.Info("Data cap usage restored from the current period", "used", humanBytes(cap.Used()))
	}

	if cfg.PprofAddr != "" {
		go func() {
			slog.Info("pprof listening", "addr", cfg.PprofAddr)
			if err := http.ListenAndServe(cfg.PprofAddr, nil); err != nil {
				slog.Warn("pprof server stopped", "error", err)
			}
		}()
	}
	if !cfg.SkipNATCheck {
		go logNATCheck(ctx, clientcore.DefaultSTUNBatchFunc)
	}
	if cfg.StatsInterval > 0 {
		go logStats(ctx, st, cap, cfg.StatsInterval)
	}

	defer func() {
		if cap != nil {
			if err := cap.persist(); err != nil {
				slog.Warn("could not persist data cap usage", "error", err)
			}
		}
	}()

	for {
		if cap != nil && isClosed(cap.Exceeded()) {
			if err := waitForNextPeriod(ctx, cap, cfg.DataCap); err != nil {
				return nil
			}
			continue
		}

		ui, err := startEngine(cfg, st, cap, rtcNet)
		if err != nil {
			return fmt.Errorf("start Unbounded engine: %w", err)
		}
		slog.Info("Serving: waiting for peers to help")

		if serve(ctx, ui, st, cap) {
			slog.Info("Unbounded volunteer peer stopped", st.snapshot().logAttrs()...)
			return nil
		}
		// The data cap was hit; loop around to wait for the next period.
	}
}

// serve blocks until ctx is done (returns true) or the data cap is reached
// (returns false). Either way the engine is stopped before returning.
func serve(ctx context.Context, ui *clientcore.UIImpl, st *stats, cap *capTracker) (done bool) {
	ticker := time.NewTicker(housekeepingInterval)
	defer ticker.Stop()
	for {
		var exceeded <-chan struct{}
		if cap != nil {
			exceeded = cap.Exceeded()
		}
		select {
		case <-ctx.Done():
			slog.Info("Stopping Unbounded engine")
			stopEngine(ui, st)
			return true
		case <-exceeded:
			slog.Warn("Data cap reached, pausing", "used", humanBytes(cap.Used()))
			stopEngine(ui, st)
			// Save the spent budget now, so a crash during the pause does
			// not restore spare budget from an earlier checkpoint.
			if err := cap.persist(); err != nil {
				slog.Warn("could not persist data cap usage", "error", err)
			}
			return false
		case <-ticker.C:
			if cap != nil {
				if cap.Rollover() {
					slog.Info("New data cap period started")
				}
				if err := cap.persist(); err != nil {
					slog.Warn("could not persist data cap usage", "error", err)
				}
			}
		}
	}
}

// engineStopGrace is how long broflake's engine waits for its workers before
// cancelling its context; the upstream chunk handler delivers its last batch
// at that cancellation. One second more covers the delivery itself.
const engineStopGrace = 11 * time.Second

// callbackQuiet is how long the chunk callbacks must stay silent after Stop
// for the engine to count as drained.
const callbackQuiet = 500 * time.Millisecond

// stopEngine stops the engine and waits for its callbacks to drain, so the
// bytes they report still reach the stats and the persisted cap. UIImpl.Stop
// returns before the engine's context is cancelled and exposes no completion
// signal, so the wait is for the callbacks to go quiet, bounded by the
// engine's own stop grace.
func stopEngine(ui *clientcore.UIImpl, st *stats) {
	ui.Stop()
	deadline := time.Now().Add(engineStopGrace)
	for time.Now().Before(deadline) {
		if time.Since(st.lastActivity()) >= callbackQuiet {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitForNextPeriod sleeps until the cap's period ends, then resets it. It
// returns ctx's error if the wait was interrupted.
func waitForNextPeriod(ctx context.Context, cap *capTracker, dc DataCap) error {
	until := cap.NextReset()
	slog.Warn("Data cap reached; not serving until the next period",
		"used", humanBytes(cap.Used()),
		"cap", dc.String(),
		"resumes", until.Format(time.RFC3339),
	)
	timer := time.NewTimer(time.Until(until))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	cap.Rollover()
	slog.Info("New data cap period started; resuming")
	return nil
}

// startEngine builds and starts a broflake widget engine. NewBroflake starts
// it as part of construction.
func startEngine(cfg Config, st *stats, cap *capTracker, rtcNet transport.Net) (*clientcore.UIImpl, error) {
	bfOpt := clientcore.NewDefaultBroflakeOptions()
	bfOpt.ClientType = "widget"
	bfOpt.CTableSize = cfg.MaxPeers
	bfOpt.PTableSize = cfg.MaxPeers
	bfOpt.Netstated = cfg.Netstated
	bfOpt.OnConnectionChangeFunc = func(state, workerIdx int, addr net.IP) {
		st.onConnectionChange(state, addr)
		// The peer's address is a censored user's IP. It stays out of the
		// service log unless debugging is on.
		switch {
		case state > 0:
			slog.Info("peer connected", "slot", workerIdx)
			slog.Debug("peer address", "slot", workerIdx, "addr", addrString(addr))
		case state < 0:
			slog.Info("peer disconnected", "slot", workerIdx)
		}
	}
	bfOpt.OnDownstreamChunkFunc = func(size, _ int) {
		st.onDownstreamChunk(size)
		if cap != nil {
			cap.add(size)
		}
	}
	bfOpt.OnUpstreamChunkFunc = func(size, _ int) {
		st.onUpstreamChunk(size)
		if cap != nil {
			cap.add(size)
		}
	}

	rtcOpt := clientcore.NewDefaultWebRTCOptions()
	rtcOpt.DiscoverySrv = cfg.Freddie
	rtcOpt.Tag = cfg.Tag
	rtcOpt.NATFailTimeout = cfg.NATFailTimeout
	rtcOpt.TargetCountries = cfg.TargetCountries
	rtcOpt.Net = rtcNet

	egOpt := clientcore.NewDefaultEgressOptions()
	egOpt.Addr = cfg.Egress

	_, ui, err := clientcore.NewBroflake(bfOpt, rtcOpt, egOpt)
	if err != nil {
		return nil, err
	}
	return ui, nil
}

// logStats writes the counters every interval until ctx is done.
func logStats(ctx context.Context, st *stats, cap *capTracker, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attrs := st.snapshot().logAttrs()
			if cap != nil {
				attrs = append(attrs, "period_used", humanBytes(cap.Used()))
			}
			slog.Info("stats", attrs...)
		}
	}
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// addrString renders a peer IP for logging, tolerating the nil addr that a
// connection-change callback may carry.
func addrString(addr net.IP) string {
	if addr == nil {
		return "(unknown)"
	}
	return addr.String()
}

// orNone renders an unset optional value as "(none)".
func orNone(v string) string {
	if v == "" {
		return "(none)"
	}
	return v
}
