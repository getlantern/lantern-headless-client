package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pterm/pterm"

	"github.com/getlantern/lantern-headless-client/shared"
	"github.com/getlantern/lantern-headless-client/unbounded"
)

// UnboundedCmd turns this host into an Unbounded volunteer peer. Every flag
// has an environment variable so a service unit can configure it from an
// EnvironmentFile.
type UnboundedCmd struct {
	Freddie         string        `arg:"--freddie,env:UNBOUNDED_FREDDIE" default:"https://freddie.iantem.io" help:"Discovery (signaling) server URL"`
	Egress          string        `arg:"--egress,env:UNBOUNDED_EGRESS" default:"wss://unbounded.iantem.io:443" help:"Egress server WebSocket URL"`
	MaxPeers        int           `arg:"--max-peers,env:UNBOUNDED_MAX_PEERS" default:"5" help:"How many censored peers to serve at once"`
	BandwidthLimit  string        `arg:"--bandwidth-limit,env:UNBOUNDED_BANDWIDTH_LIMIT" help:"Byte rate cap per direction, e.g. 5mbit, 500KB/s, 1.5MiB/s (default: unlimited)"`
	DataCap         string        `arg:"--data-cap,env:UNBOUNDED_DATA_CAP" help:"Pause once this much has been relayed in a period, e.g. 50GB/day, 1TB/month (default: none)"`
	TargetCountries string        `arg:"--target-countries,env:UNBOUNDED_TARGET_COUNTRIES" help:"Comma-separated ISO 3166-1 alpha-2 countries whose users to prefer, e.g. IR,RU"`
	Tag             string        `arg:"--tag,env:UNBOUNDED_TAG" help:"Label for this peer in the swarm's network view"`
	Netstated       string        `arg:"--netstated,env:UNBOUNDED_NETSTATED" help:"Network-state reporting endpoint"`
	NATFailTimeout  time.Duration `arg:"--nat-fail-timeout,env:UNBOUNDED_NAT_FAIL_TIMEOUT" default:"15s" help:"How long a WebRTC attempt may spend in NAT traversal"`
	StatsInterval   time.Duration `arg:"--stats-interval,env:UNBOUNDED_STATS_INTERVAL" default:"60s" help:"How often to log relay statistics (0 disables)"`
	SkipNATCheck    bool          `arg:"--skip-nat-check,env:UNBOUNDED_SKIP_NAT_CHECK" help:"Skip the startup NAT mapping probe"`
	PprofAddr       string        `arg:"--pprof-addr,env:UNBOUNDED_PPROF_ADDR" help:"Serve net/http/pprof on this address"`
}

// runUnbounded runs the volunteer peer until SIGINT, SIGTERM or SIGQUIT and
// returns the process exit code.
func runUnbounded(cmd *UnboundedCmd, logFile io.Writer) int {
	cfg, err := unboundedConfig(cmd)
	if err != nil {
		pterm.Error.Printfln("Invalid Unbounded options: %s", err)
		return 2
	}

	// broflake logs through slog only. Send it to the log file, and to the
	// terminal unless --quiet.
	var out io.Writer = logFile
	if !args.Quiet {
		out = io.MultiWriter(logFile, os.Stderr)
	}
	unbounded.SetupLogging(out, args.Debug)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	pterm.Info.Printfln("Starting Unbounded volunteer peer... Log file: %s", logFilePath)
	if err := unbounded.Run(ctx, cfg); err != nil {
		pterm.Error.Printfln("Unbounded volunteer peer failed: %s", err)
		return 1
	}
	return 0
}

func unboundedConfig(cmd *UnboundedCmd) (unbounded.Config, error) {
	rate, err := unbounded.ParseRate(cmd.BandwidthLimit)
	if err != nil {
		return unbounded.Config{}, fmt.Errorf("--bandwidth-limit: %w", err)
	}
	dataCap, err := unbounded.ParseDataCap(cmd.DataCap)
	if err != nil {
		return unbounded.Config{}, fmt.Errorf("--data-cap: %w", err)
	}
	var countries []string
	for _, c := range strings.Split(cmd.TargetCountries, ",") {
		if c = strings.ToUpper(strings.TrimSpace(c)); c != "" {
			countries = append(countries, c)
		}
	}
	cfg := unbounded.Config{
		Freddie:         cmd.Freddie,
		Egress:          cmd.Egress,
		MaxPeers:        cmd.MaxPeers,
		BandwidthLimit:  rate,
		DataCap:         dataCap,
		TargetCountries: countries,
		Tag:             cmd.Tag,
		Netstated:       cmd.Netstated,
		NATFailTimeout:  cmd.NATFailTimeout,
		StatsInterval:   cmd.StatsInterval,
		SkipNATCheck:    cmd.SkipNATCheck,
		PprofAddr:       cmd.PprofAddr,
		DataPath:        args.DataPath,
		Version:         shared.ApplicationVersion,
	}
	return cfg, cfg.Validate()
}
