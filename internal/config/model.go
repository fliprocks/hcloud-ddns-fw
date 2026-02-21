package config

import (
	"log/slog"
	"os"

	"github.com/goccy/go-yaml"
)

type Record struct {
	RecordType string `yaml:"type"`
	Name string `yaml:"name"`
	TTL int `yaml:"ttl"`
	Comment string `yaml:"comment"`
}

type DNSZone struct {
	Name string `yaml:"zone-name"`
	Records []Record `yaml:"records"`
}

type Rule struct {
	Description string `yaml:"description"`
	Direction string `yaml:"direction"`
    Port int `yaml:"port"`
	Protocol string `yaml:"protocol"`
	IPv4mask int `yaml:"ipv4mask"`
	IPv6mask int `yaml:"ipv6mask"`
}

type Firewall struct {
	ID int64 `yaml:"id"`
	Rules []Rule `yaml:"fw-rules"`
}


type Config struct {
	DNSZones []DNSZone `yaml:"dns-zones"`
	Firewalls []Firewall `yaml:"firewalls"`
}


func NewConfig(logger *slog.Logger) (*Config, error) {
	var cfg Config
	p := "config.yaml"

	if os.Getenv("DEVMODE") == "true" {
		p = "config-dev.yaml"
		logger.Info("Useing development config", "path", p)
	}

	b, err := os.ReadFile(p)
	if err != nil {
		logger.Error("Unable to read config file",
			"path", p,
			"err", err,
		)
		return nil, err
	}

	if err := yaml.Unmarshal(b, &cfg); err != nil {
		logger.Error("Unable to read unmarshal config file",
			"path", p,
			"err", err,
		)
		return nil, err
	}

	return &cfg, nil
}

