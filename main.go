package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/alexflint/go-arg"
	"github.com/getlantern/appdir"
	"github.com/getlantern/flashlight/v7"
	"github.com/getlantern/flashlight/v7/client"
	"github.com/getlantern/flashlight/v7/common"
	"github.com/getlantern/flashlight/v7/stats"
	"github.com/getlantern/golog"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/getlantern/lantern-headless-client/deviceid"
)

type AuthCmd struct {
	UserID int64  `arg:"--user-id,required" placeholder:"UID" help:"User ID" toml:"user_id"`
	Token  string `arg:"--token,required" placeholder:"TOKEN" help:"User Token" toml:"token"`
}

type ServeCmd struct {
	HTTPProxyAddress  string `arg:"--http-proxy-addr" default:"127.0.0.1:0" help:"Address to bind to and use for HTTP the proxy. Defaults to a random port on localhost"`
	SOCKSProxyAddress string `arg:"--socks-proxy-addr" default:"127.0.0.1:0" help:"Address to bind to and use for SOCKS the proxy. Defaults to a random port on localhost"`
	StickyConfig      bool   `arg:"--sticky-config" help:"Whether to use a sticky config"`
	ReadableConfig    bool   `arg:"--readable-config" help:"Whether to use a readable config"`
	ProxiesYaml       string `arg:"--proxies-yaml" help:"Path to a custom proxies.yaml file. Assumes --sticky-config"`
	DeviceId          string `arg:"--device-id" help:"Custom Device ID"`
}

var args struct {
	Auth  *AuthCmd  `arg:"subcommand:auth" help:"Authenticate with Lantern"`
	Serve *ServeCmd `arg:"subcommand:start" help:"Start Lantern"`

	DataPath string     `arg:"--data-path" help:"Path to store data (config, logs, etc)"`
	LogLevel slog.Level `arg:"--log-level,env:LOG_LEVEL" default:"INFO" help:"Minimum log level to output. Value should be DEBUG, INFO, WARN, or ERROR."`
	JSON     bool       `arg:"--json" help:"Output logs in JSON format"`
}

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
		slog.Info("Proxy is ready to receive connections on", "socks_addr", sa, "https_addr", ha)

	}, func(err error) {
		slog.Error("Failed to start Lantern", "error", err)
	})
}

func setupLogging() {
	opts := &slog.HandlerOptions{
		Level: args.LogLevel,
	}
	var handler slog.Handler
	if args.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

var configFilePath, logFilePath string

func setupDataFolder() {
	if args.DataPath == "" {
		args.DataPath = filepath.Join(appdir.InHomeDir(".lantern-headless"))
	}

	if err := os.Mkdir(args.DataPath, 0755); err != nil && !os.IsExist(err) {
		slog.Error("Unable to create folder to store data, defaulting to current directory", "error", err)
		args.DataPath = "."
	}
	configFilePath = filepath.Join(args.DataPath, "config.toml")
	logFilePath = filepath.Join(args.DataPath, "lantern-headless.log")
}

func readAuth() {
	if _, err := toml.DecodeFile(configFilePath, &args.Auth); err != nil {
		args.Auth = &AuthCmd{UserID: 0, Token: ""}
	}
}

func auth(cmd *AuthCmd) {
	f, err := os.Create(configFilePath)
	if err != nil {
		slog.Error("Unable to create config file", "error", err)
		os.Exit(2)
	}
	defer f.Close()
	if err = toml.NewEncoder(f).Encode(cmd); err != nil {
		slog.Error("Unable to write config file", "error", err)
		os.Exit(2)
	}
	slog.Info("Config file written successfully")
}

func main() {
	argParser := arg.MustParse(&args)
	setupLogging()
	setupDataFolder()
	switch {
	case args.Auth != nil:
		auth(args.Auth)
		break
	case args.Serve != nil:
		readAuth()
		serve(args.Serve)
		break
	default:
		argParser.WriteHelp(os.Stdout)
	}
}
