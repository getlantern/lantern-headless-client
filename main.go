package main

import (
	"context"
	"fmt"
	"github.com/getlantern/golog"
	"github.com/pterm/pterm"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/getlantern/appdir"
)

func setupOutput() {
	if args.Debug {
		pterm.EnableDebugMessages()
	}
	if args.Quiet {
		pterm.DisableDebugMessages()
	}
	if args.Raw {
		pterm.DisableColor()
	}
}

func setupLog() *lumberjack.Logger {
	logWriter := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // megabytes
		MaxBackups: 5,
	}

	golog.SetOutputs(logWriter, logWriter)
	golog.SetPrepender(func(writer io.Writer) {
		_, _ = writer.Write([]byte(fmt.Sprintf("%s: ", time.Now().Format("2006-01-02 15:04:05"))))
	})
	return logWriter
}

func setupDataFolder() {
	if args.DataPath == "" {
		args.DataPath = filepath.Join(appdir.InHomeDir(".lantern-headless"))
	}

	if err := os.Mkdir(args.DataPath, 0755); err != nil && !os.IsExist(err) {
		pterm.Error.Printfln("Unable to create folder to store data, defaulting to current directory. Error: %s", err.Error())
		args.DataPath = "."
	}
	configFilePath = filepath.Join(args.DataPath, "config.toml")
	logFilePath = filepath.Join(args.DataPath, "lantern-headless.log")
}

func main() {
	argParser := arg.MustParse(&args)
	setupOutput()
	setupDataFolder()
	logWriter := setupLog()
	defer func(logWriter *lumberjack.Logger) {
		_ = logWriter.Close()
	}(logWriter)
	ctx := context.Background()
	switch {
	case args.Auth != nil:
		authCmd(ctx, args.Auth, args.AuthURL, logWriter)
		break
	case args.Serve != nil:
		readAuth()
		serve(args.Serve)
		break
	default:
		argParser.WriteHelp(pterm.DefaultLogger.Writer)
	}
}
