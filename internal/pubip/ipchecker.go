package pubip

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type IPChecker struct {
	logger      *slog.Logger
	IPv4        string
	IPv6        string
	HttpTimeout int64
}

func NewIPChecker(logger *slog.Logger) (*IPChecker, error) {

	timeout, err := strconv.ParseInt(os.Getenv("HTTP_TIMEOUT"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("Invalid HTTP_TIMEOUT: %w", err)
	}

	return &IPChecker{
		logger:      logger,
		HttpTimeout: timeout,
	}, nil
}

func (u *IPChecker) checkIP(apiURL string) (string, error) {
	client := &http.Client{
		Timeout: time.Duration(u.HttpTimeout) * time.Second,
	}

	res, err := client.Get(apiURL)
	if err != nil {
		u.logger.Error("Error while fetching current IP address", "err", err)
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unexpected HTTP status %d from %s", res.StatusCode, apiURL)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		u.logger.Error("Error reading the IP response", "err", err)
		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}

func (u *IPChecker) writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func (u *IPChecker) compareIPs(path, ip string) (bool, error) {
	_, err := os.Stat(path)

	if err == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return false, err
		}

		s := strings.TrimSpace(string(b))
		if ip == s {
			return false, nil
		}

		if err := u.writeFile(path, ip); err != nil {
			return false, err
		}
		u.logger.Info("Updated IP address found", "ip", ip)
		return true, nil
	}

	if os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return false, err
		}

		if err := u.writeFile(path, ip); err != nil {
			return false, err
		}
		u.logger.Info("New IP address stored", "ip", ip)
		return true, nil
	}

	return false, err
}

func stateDir() string {
	if d := strings.TrimSpace(os.Getenv("IP_STATE_DIR")); d != "" {
		return d
	}

	return "/tmp/hcloud-ddns-fw"
}

func (u *IPChecker) CheckForChanges() (changed bool, err error) {
	var ipv4 bool
	var ipv6 bool

	switch strings.ToLower(os.Getenv("IP_MODE")) {
	case "ipv4":
		ipv4 = true
	case "ipv6":
		ipv6 = true
	case "dual":
		ipv4 = true
		ipv6 = true
	default:
		return false, fmt.Errorf("invalid IP_MODE: %q (use IPv4, IPv6, or Dual)", os.Getenv("IP_MODE"))
	}

	var changedV4 bool
	var changedV6 bool

	if ipv4 {
		ip, err := u.checkIP("https://api.ipify.org")
		if err != nil {
			return false, err
		}

		changedV4, err = u.compareIPs(filepath.Join(stateDir(), "ipv4"), ip)
		if err != nil {
			return false, err
		}
		u.IPv4 = ip
	}

	if ipv6 {
		ip, err := u.checkIP("https://api6.ipify.org")
		if err != nil {
			return false, err
		}

		changedV6, err = u.compareIPs(filepath.Join(stateDir(), "ipv6"), ip)
		if err != nil {
			return false, err
		}
		u.IPv6 = ip
	}

	if changedV4 || changedV6 {
		changed = true
	}

	return
}
