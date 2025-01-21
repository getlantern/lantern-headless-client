package main

import (
	"fmt"
	"log/slog"
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

type Args struct {
	Auth  *AuthCmd  `arg:"subcommand:auth" help:"Authenticate with Lantern"`
	Serve *ServeCmd `arg:"subcommand:start" help:"Start Lantern"`

	DataPath string     `arg:"--data-path" help:"Path to store data (config, logs, etc)"`
	LogLevel slog.Level `arg:"--log-level,env:LOG_LEVEL" default:"INFO" help:"Minimum log level to output. Value should be DEBUG, INFO, WARN, or ERROR."`
	JSON     bool       `arg:"--json" help:"Output logs in JSON format"`
}

func (Args) Version() string {
	return fmt.Sprintf("lantern-headless v%s [Build Date: %s Revision Date: %s]", ApplicationVersion, BuildDate, RevisionDate)
}

var args Args
