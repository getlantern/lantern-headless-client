package main

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/getlantern/flashlight/v7"
	"github.com/getlantern/flashlight/v7/client"
	"github.com/getlantern/flashlight/v7/common"
	"github.com/getlantern/flashlight/v7/stats"
	"github.com/getlantern/golog"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/getlantern/lantern-headless-client/deviceid"
)

func serve(cmd *ServeCmd) {
	if cmd.DeviceId == "" {
		cmd.DeviceId = deviceid.Get()
	}

	logWriter := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // megabytes
		MaxBackups: 5,
	}

	golog.SetOutputs(logWriter, logWriter)
	golog.SetPrepender(func(writer io.Writer) {
		_, _ = writer.Write([]byte(fmt.Sprintf("%s: ", time.Now().Format("2006-01-02 15:04:05"))))
	})

	settings := common.NewUserConfigData("lantern-headless", cmd.DeviceId, args.Auth.UserID, args.Auth.Token, nil, "en-US")
	statsTracker := stats.NewTracker()
	var onOneProxy sync.Once
	proxyReady := make(chan struct{})
	slog.Info("Starting Lantern...", "log", logFilePath)
	fc, err := flashlight.New(
		"lantern-headless",
		"999.999.999",
		"10-10-2024",
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
				slog.Info("Successfully dialed the proxy")
				proxyReady <- struct{}{}
			})
		}),
	)
	if err != nil {
		slog.Error("Failed to initialize Lantern", "error", err)
		return
	}

	fc.Run(cmd.HTTPProxyAddress, cmd.SOCKSProxyAddress, func(cl *client.Client) {
		slog.Info("Waiting for proxy to be ready")
		sa, ok := cl.Socks5Addr(5 * time.Second)
		if !ok {
			slog.Error("Failed to get proxy address")
			return
		}
		ha, ok := cl.Addr(5 * time.Second)
		if !ok {
			slog.Error("Failed to get proxy address")
			return
		}
		select {
		case <-proxyReady:
			break
		}
		slog.Info("Proxy is ready to receive connections on", "socks_addr", sa, "http_addr", ha)

	}, func(err error) {
		slog.Error("Failed to start Lantern", "error", err)
	})
}
