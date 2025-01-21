package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/alexflint/go-arg"
	"github.com/getlantern/appdir"
)

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
