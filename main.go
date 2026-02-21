package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/fliprocks/hcloud-ddns-fw/internal"
	"github.com/fliprocks/hcloud-ddns-fw/internal/config"
	"github.com/fliprocks/hcloud-ddns-fw/internal/hcloud_functions"
	"github.com/fliprocks/hcloud-ddns-fw/internal/pubip"
)

func main() {

	logger := config.NewLogger()
	logger.Info("System started...")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "list-firewalls":
			logger.Info("Executing command", "cmd", os.Args[1])
			firewalls, err := hcloud_functions.ListFirewalls()

			if err != nil {
				logger.Error("Error executing command", "err", err)
				os.Exit(0)
			}

			fmt.Println("Firewalls:")
			for _, fw := range firewalls {
				fmt.Printf("ID: %d | Name: %s\n", fw.ID, fw.Name)
			}

			os.Exit(0)

		default:
			logger.Error("unknown command", "cmd", os.Args[1])
			os.Exit(0)
		}
	}

	if os.Getenv("HCLOUD_TOKEN") == "" {
		logger.Error("No HCLOUD_TOKEN provided. Stopping service...")
		os.Exit(0)
	}

	if os.Getenv("IP_MODE") == "" {
		logger.Error("No IP_MODE provided. Stopping service...")
		os.Exit(0)
	}

	cfg, err := config.NewConfig(logger)
	if err != nil {
		logger.Error("Stopping service due to errors in the config file", "err", err)
		os.Exit(0)
	}
	logger.Debug("Config", "cfg", cfg)

	ipc, err := pubip.NewIPChecker(logger)
  	if err != nil {
		logger.Error("Error creating IPChecker", "err", err)
		os.Exit(0)
    }

	upd, err := internal.NewUpdater(logger, cfg, ipc)
	if err != nil {
		logger.Error("Error creating Updater", "err", err)
		os.Exit(0)
    }

	i, err := strconv.ParseInt(os.Getenv("UPDATE_INTERVAL"), 10, 64)
    if err != nil {
		logger.Error("Invalid UPDATE_INTERVAL", "err", err)
		os.Exit(0)
    }

	success := false

	for true {
		changed, err := ipc.CheckForChanges()

		if err != nil {
			logger.Error("Stopping service due to errors", "err", err)
			os.Exit(0)
		}
		
		if changed || !success {
			logger.Info("Detected changed IP address. Starting update process")
			success = upd.Update()
		}

		logger.Debug("Sleeping", "duration", time.Duration(i)*time.Second)
		time.Sleep(time.Duration(i) * time.Second)
	}
}
