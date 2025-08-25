package main

import (
	"fmt"

	"github.com/alexflint/go-arg"
	"github.com/pterm/pterm"

	"github.com/getlantern/lantern-headless-client/shared"
)

type ServeCmd struct {
	HTTPProxyAddress  string `arg:"--http-proxy-addr" default:"127.0.0.1:0" help:"Address to bind to and use for HTTP the proxy. Defaults to a random port on localhost"`
	SOCKSProxyAddress string `arg:"--socks-proxy-addr" default:"127.0.0.1:0" help:"Address to bind to and use for SOCKS the proxy. Defaults to a random port on localhost"`
	StickyConfig      bool   `arg:"--sticky-config" help:"Whether to use a sticky config"`
	ReadableConfig    bool   `arg:"--readable-config" help:"Whether to use a readable config"`
}

type AuthCmd struct {
	Login  *LoginCmd  `arg:"subcommand:login" help:"Login to Lantern"`
	Signup *SignupCmd `arg:"subcommand:signup" help:"Signup for Lantern"`
	Logout *LogoutCmd `arg:"subcommand:logout" help:"Logout of Lantern"`
}

type LoginCmd struct {
	Email    string `arg:"--email,env:EMAIL" help:"Email address"`
	Password string `arg:"--password,env:PASSWORD" help:"Password"`
}

type SignupCmd struct {
	Email    string `arg:"--email,env:EMAIL" help:"Email address"`
	Password string `arg:"--password,env:PASSWORD" help:"Password"`
}

type LogoutCmd struct {
}

type Args struct {
	// sub commands
	Auth  *AuthCmd  `arg:"subcommand:auth" help:"Authenticate with Lantern"`
	Serve *ServeCmd `arg:"subcommand:start" help:"Start Lantern"`

	// common options
	DataPath string `arg:"--data-path" help:"Path to store data (config, logs, etc)"`
	Debug    bool   `arg:"--debug" help:"Enable debug logging"`
	Quiet    bool   `arg:"--quiet" help:"Disable logging to stdout"`
	Raw      bool   `arg:"--raw" help:"Output logs without any formatting"`
	AuthURL  string `arg:"--auth-url" help:"Override URL of the authentication server"`
	Insecure bool   `arg:"--insecure" help:"Whether to skip TLS verification"`
}

func (Args) Version() string {
	return fmt.Sprintf("lantern-headless v%s [Build Date: %s Revision Date: %s]", shared.ApplicationVersion, shared.BuildDate, shared.RevisionDate)
}

var args Args
var userID int64
var userToken string
var deviceId string
var configFilePath, logFilePath string
var argParser = arg.MustParse(&args)

func printHelp() {
	argParser.WriteHelp(pterm.DefaultLogger.Writer)
}
