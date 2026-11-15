package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
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
				os.Exit(1)
			}

			fmt.Println("Firewalls:")
			for _, fw := range firewalls {
				fmt.Printf("ID: %d | Name: %s\n", fw.ID, fw.Name)
			}

			os.Exit(0)

		default:
			logger.Error("unknown command", "cmd", os.Args[1])
			os.Exit(1)
		}
	}

	if os.Getenv("HCLOUD_TOKEN") == "" {
		logger.Error("No HCLOUD_TOKEN provided. Stopping service...")
		os.Exit(1)
	}

	trigger := strings.ToLower(strings.TrimSpace(os.Getenv("TRIGGER")))
	if trigger == "" {
		trigger = internal.TriggerInterval
		logger.Info("No TRIGGER provided. Use default setting \"interval\" as trigger")
	}
	if trigger != internal.TriggerInterval && trigger != internal.TriggerUpdateURL {
		logger.Error("Invalid TRIGGER (use interval or updateUrl). Stopping service...", "TRIGGER", os.Getenv("TRIGGER"))
		os.Exit(1)
	}

	ipSource := strings.ToLower(strings.TrimSpace(os.Getenv("IP_ADDRESS_SRC")))
	if ipSource == "" {
		ipSource = internal.IPSourceAPIURL
	}
	if ipSource != internal.IPSourceAPIURL && ipSource != internal.IPSourceUpdateURL {
		logger.Error("Invalid IP_ADDRESS_SRC (use apiUrl or updateUrl). Stopping service...", "IP_ADDRESS_SRC", os.Getenv("IP_ADDRESS_SRC"))
		os.Exit(1)
	}
	if ipSource == internal.IPSourceUpdateURL && trigger != internal.TriggerUpdateURL {
		logger.Error("IP_ADDRESS_SRC updateUrl requires TRIGGER updateUrl. Stopping service...")
		os.Exit(1)
	}

	if os.Getenv("IP_MODE") == "" {
		logger.Error("No IP_MODE provided. Stopping service...")
		os.Exit(1)
	}

	if ipSource == internal.IPSourceAPIURL {
		if os.Getenv("IPv4_API_URL") == "" {
			logger.Info("No IPv4_API_URL provided. Use default setting \"https://api.ipify.org\" as IPv4 API URL")
		}

		if os.Getenv("IPv6_API_URL") == "" {
			logger.Info("No IPv6_API_URL provided. Use default setting \"https://api6.ipify.org\" as IPv6 API URL")
		}
	}

	cfg, err := config.NewConfig(logger)
	if err != nil {
		logger.Error("Stopping service due to errors in the config file", "err", err)
		os.Exit(1)
	}
	logger.Debug("Config", "cfg", cfg)

	ipc, err := pubip.NewIPChecker(logger)
	if err != nil {
		logger.Error("Error creating IPChecker", "err", err)
		os.Exit(1)
	}

	upd, err := internal.NewUpdater(logger, cfg, ipc)
	if err != nil {
		logger.Error("Error creating Updater", "err", err)
		os.Exit(1)
	}

	switch trigger {
	case internal.TriggerInterval:
		i, err := strconv.ParseInt(os.Getenv("UPDATE_INTERVAL"), 10, 64)
		if err != nil || i <= 0 {
			logger.Error("Invalid UPDATE_INTERVAL. Stopping service...", "UPDATE_INTERVAL", os.Getenv("UPDATE_INTERVAL"))
			os.Exit(1)
		}

		err = internal.RunInterval(logger, upd, ipc, time.Duration(i)*time.Second)
		logger.Error("Stopping service due to errors", "err", err)
		os.Exit(1)

	case internal.TriggerUpdateURL:
		user := os.Getenv("UPDATE_URL_USER")
		pass := os.Getenv("UPDATE_URL_PASS")
		if user == "" || pass == "" {
			logger.Error("TRIGGER updateUrl requires UPDATE_URL_USER and UPDATE_URL_PASS. Stopping service...")
			os.Exit(1)
		}

		listen := strings.TrimSpace(os.Getenv("UPDATE_URL_LISTEN"))
		if listen == "" {
			listen = ":8080"
		}
		if !strings.Contains(listen, ":") {
			listen = ":" + listen // bare port, e.g. "8080"
		}
		host, port, err := net.SplitHostPort(listen)
		if p, perr := strconv.Atoi(port); err != nil || perr != nil || p < 1 || p > 65535 {
			logger.Error("Invalid UPDATE_URL_LISTEN (use <ip>:<port> or :<port>). Stopping service...", "UPDATE_URL_LISTEN", listen)
			os.Exit(1)
		}
		if host != "" && net.ParseIP(host) == nil {
			logger.Error("Invalid IP address in UPDATE_URL_LISTEN. Stopping service...", "UPDATE_URL_LISTEN", listen)
			os.Exit(1)
		}

		ipMode := strings.ToLower(os.Getenv("IP_MODE"))
		if ipSource == internal.IPSourceUpdateURL && (ipMode == "ipv6" || ipMode == "dual") && os.Getenv("IPv6_INTERFACE_ID") == "" {
			logger.Error("IP_ADDRESS_SRC updateUrl with IPv6 requires IPv6_INTERFACE_ID. Stopping service...", "IP_MODE", os.Getenv("IP_MODE"))
			os.Exit(1)
		}

		ipv6 := internal.IPv6Options{
			PrefixID:    os.Getenv("IPv6_PREFIX_ID"),
			InterfaceID: os.Getenv("IPv6_INTERFACE_ID"),
		}
		if v := os.Getenv("IPv6_PREFIX_LENGTH"); v != "" {
			l, err := strconv.Atoi(v)
			if err != nil || l < 1 || l > 64 {
				logger.Error("Invalid IPv6_PREFIX_LENGTH (1-64). Stopping service...", "IPv6_PREFIX_LENGTH", v)
				os.Exit(1)
			}
			ipv6.PrefixLength = l
		}

		srv := internal.NewUpdateURLServer(logger, upd, ipc, user, pass, ipSource, ipv6)
		srv.InitialUpdate()
		err = srv.ListenAndServe(listen)
		logger.Error("Stopping service due to errors", "err", err)
		os.Exit(1)
	}
}
