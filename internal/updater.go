package internal

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/fliprocks/hcloud-ddns-fw/internal/config"
	"github.com/fliprocks/hcloud-ddns-fw/internal/hcloud_functions"
	"github.com/fliprocks/hcloud-ddns-fw/internal/pubip"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type Updater struct {
	logger *slog.Logger
	cfg *config.Config
	ipc *pubip.IPChecker
	token string
	httpTimeout int64
}

type Firewall struct {
	Rules []hcloud.FirewallRule
	FWID int64
}

func NewUpdater(logger *slog.Logger, config *config.Config, ipchecker *pubip.IPChecker) (*Updater, error) {
	token := os.Getenv("HCLOUD_TOKEN")
	timeout, err := strconv.ParseInt(os.Getenv("HTTP_TIMEOUT"), 10, 64)
	if err != nil {
		return nil, err
    }

	return &Updater{
		logger: logger,
		cfg: config,
		ipc: ipchecker,
		token: token,
		httpTimeout: timeout,
	}, nil
}

func (u *Updater) Update() bool {

	success := true

	for i := 0; i < len(u.cfg.DNSZones); i++ {

		u.logger.Debug("Try to update DNS zone", "zone", u.cfg.DNSZones[i].Name)
		u.logger.Debug("Use IPs",
			"IPv4", u.ipc.IPv4,
			"IPv6", u.ipc.IPv6,
		)

		for j := 0; j < len(u.cfg.DNSZones[i].Records); j++ {

			u.logger.Debug("Try to update record", 
				"type", u.cfg.DNSZones[i].Records[j].RecordType,
				"name", u.cfg.DNSZones[i].Records[j].Name,
				"ttl", u.cfg.DNSZones[i].Records[j].TTL,
				"comment", u.cfg.DNSZones[i].Records[j].Comment,
			)

			ipm := os.Getenv("IP_MODE")

			if u.cfg.DNSZones[i].Records[j].RecordType == "AAAA" && ipm == "IPv4" {
				u.logger.Error("Error in config file. Stopping service...", "IP_MODE", ipm, "RecordType", u.cfg.DNSZones[i].Records[j].RecordType)
				os.Exit(0)
			}

			if u.cfg.DNSZones[i].Records[j].RecordType == "A" && ipm == "IPv6" {
				u.logger.Error("Error in config file. Stopping service...", "IP_MODE", ipm, "RecordType", u.cfg.DNSZones[i].Records[j].RecordType)
				os.Exit(0)
			}

			res, errDns := hcloud_functions.SetRecord(
				u.cfg.DNSZones[i],
				u.cfg.DNSZones[i].Records[j],
				u.ipc.IPv4,
				u.ipc.IPv6,
				u.token,
				u.httpTimeout,
			)

			if errDns != nil {
				u.logger.Error("Failed to update DNS record", "err", errDns)
				success = false
				if 400 <= res.StatusCode && res.StatusCode < 500 {
					u.logger.Error("Invalid config file. Stopping service...")
					os.Exit(0)
				}
			} else {
				u.logger.Debug("Sucessfully updated DNS record", 
					"name", u.cfg.DNSZones[i].Records[j].Name,
					"zone", u.cfg.DNSZones[i],
				)
			}

			res, errTtl := hcloud_functions.SetTTL(
				u.cfg.DNSZones[i],
				u.cfg.DNSZones[i].Records[j],
				u.token,
				u.httpTimeout,
			)

			if errTtl != nil {
				u.logger.Error("Failed to update TTL", "err", errTtl)
				success = false
				if 400 <= res.StatusCode && res.StatusCode < 500 {
					u.logger.Error("Invalid config file. Stopping service...")
					os.Exit(0)
				}
			} else {
				u.logger.Debug("Sucessfully updated TTL",
					"name", u.cfg.DNSZones[i].Records[j].Name,
					"zone", u.cfg.DNSZones[i],
				)
			}
		}

		if success {
			u.logger.Info("Sucessfully updated DNS zone", "zone", u.cfg.DNSZones[i].Name)
		}
	}

	for i := 0; i < len(u.cfg.Firewalls); i++ {

		var fw Firewall
		fw.FWID = u.cfg.Firewalls[i].ID

		u.logger.Debug("Try to create Firewall config", 
			"id", u.cfg.Firewalls[i].ID,
		)

		u.logger.Debug("Use IPs",
			"IPv4", u.ipc.IPv4,
			"IPv6", u.ipc.IPv6,
		)

		for j := 0; j < len(u.cfg.Firewalls[i].Rules); j++ {

			u.logger.Debug("Try to create FW rule",
				"description", u.cfg.Firewalls[i].Rules[j].Description,
				"direction", u.cfg.Firewalls[i].Rules[j].Direction,
				"port", u.cfg.Firewalls[i].Rules[j].Port,
				"protocol", u.cfg.Firewalls[i].Rules[j].Protocol,
				"ipv4mask", u.cfg.Firewalls[i].Rules[j].IPv4mask,
				"ipv6mask", u.cfg.Firewalls[i].Rules[j].IPv6mask,
			)

			r, errFw := hcloud_functions.CreateFWFule(
				u.cfg.Firewalls[i].Rules[j],
				u.ipc.IPv4,
				u.ipc.IPv6,
			)

			if errFw != nil {
				u.logger.Error("Failed to create firewall rule", "err", errFw)
				success = false
			} else {
				u.logger.Debug("Sucessfully created firewall rule", 
					"description", u.cfg.Firewalls[i].Rules[j].Description,
					"port", u.cfg.Firewalls[i].Rules[j].Port,
					"protocol", u.cfg.Firewalls[i].Rules[j].Protocol,
				)
			}

			fw.Rules = append(fw.Rules, r)
		}

		res, err := hcloud_functions.UpdateFw(
			fw.Rules,
			fw.FWID,
			u.token,
			u.httpTimeout,
		)

		if err != nil {
			u.logger.Error("Failed to update Firewall", "err", err)
			success = false
			if 400 <= res.StatusCode && res.StatusCode < 500 {
				u.logger.Error("Invalid config file. Stopping service...")
				os.Exit(0)
			}
		}

		if success {
			u.logger.Info("Sucessfully updated Firewall", "firewall", u.cfg.Firewalls[i].ID)
		}
	}

	return success
}

