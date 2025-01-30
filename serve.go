package main

import (
	"github.com/pterm/pterm"
	"sync"
	"time"

	"github.com/getlantern/flashlight/v7"
	"github.com/getlantern/flashlight/v7/client"
	"github.com/getlantern/flashlight/v7/common"
	"github.com/getlantern/flashlight/v7/stats"

	"github.com/getlantern/lantern-headless-client/shared"
)

var spinner = pterm.DefaultSpinner.WithRemoveWhenDone(true).WithDelay(200 * time.Millisecond)

func serve(cmd *ServeCmd) {
	settings := common.NewUserConfigData("lantern-headless", deviceId, userID, userToken, nil, "en-US")
	statsTracker := stats.NewTracker()
	var onOneProxy sync.Once
	proxyReady := make(chan struct{})
	pterm.Info.Printfln("Starting Lantern... Log file: %s", logFilePath)
	fc, err := flashlight.New(
		"lantern-headless",
		shared.ApplicationVersion,
		shared.RevisionDate,
		args.DataPath,
		false,
		func() bool { return false },
		func() bool { return false },
		func() bool { return false },
		func() bool { return false },
		map[string]interface{}{
			"readableconfig": cmd.ReadableConfig,
			"stickyconfig":   cmd.StickyConfig,
		},
		settings,
		statsTracker,
		func() bool { return false },
		func() string { return "en-US" },
		func(host string) (string, error) {
			return host, nil
		},
		func(category, action, label string) {
		},
		flashlight.WithOnSucceedingProxy(func() {
			onOneProxy.Do(func() {
				pterm.Success.Printfln("Successfully dialed the proxy")
				proxyReady <- struct{}{}
			})
		}),
	)
	if err != nil {
		pterm.Error.Printfln("Failed to initialize Lantern. Error: %s", err)
		return
	}

	fc.Run(cmd.HTTPProxyAddress, cmd.SOCKSProxyAddress, func(cl *client.Client) {
		spin, _ := spinner.Start("Waiting for proxy to be ready")
		defer spin.Stop()
		sa, ok := cl.Socks5Addr(5 * time.Second)
		if !ok {
			pterm.Error.Println("Failed to get proxy address")
			return
		}
		ha, ok := cl.Addr(5 * time.Second)
		if !ok {
			pterm.Error.Println("Failed to get proxy address")
			return
		}
		select {
		case <-proxyReady:
			break
		}
		pterm.Info.Printfln("Proxy is ready to receive connections on socks_addr: %v http_addr: %v", sa, ha)

	}, func(err error) {
		pterm.Error.Printfln("Failed to start Lantern, error: %s", err)
	})
}
