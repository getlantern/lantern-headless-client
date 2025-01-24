package main

import (
	"log/slog"
	"os"

	"github.com/BurntSushi/toml"
)

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
