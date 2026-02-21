package hcloud_functions

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fliprocks/hcloud-ddns-fw/internal/config"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func ListFirewalls() ([]*hcloud.Firewall, error) {
	token := os.Getenv("HCLOUD_TOKEN")

	client := hcloud.NewClient(hcloud.WithToken(token))

	ctx, cancel := context.WithTimeout(context.Background(), 15 * time.Second)
	defer cancel()

	firewalls, err := client.Firewall.All(ctx)
	if err != nil {
		return nil, err
	}

	return firewalls, nil
}

func mapDirAndProto(d, p string) (hcloud.FirewallRuleDirection, hcloud.FirewallRuleProtocol, error) {

	var dir hcloud.FirewallRuleDirection
	var proto hcloud.FirewallRuleProtocol

	switch strings.ToUpper(d) {
	case "IN":
		dir = hcloud.FirewallRuleDirectionIn
	case "OUT":
		dir = hcloud.FirewallRuleDirectionOut
	default:
		return "", "", fmt.Errorf("unsupported firewall direction: %s", dir)
	}

	switch strings.ToUpper(p) {
	case "TCP":
		proto = hcloud.FirewallRuleProtocolTCP
	case "UDP":
		proto = hcloud.FirewallRuleProtocolUDP
	case "ICMP":
		proto = hcloud.FirewallRuleProtocolICMP
	case "GRE":
		proto = hcloud.FirewallRuleProtocolGRE
	case "ESP":
		proto = hcloud.FirewallRuleProtocolESP
	default:
		return "", "", fmt.Errorf("unsupported firewall protocol: %s", proto)
	}

	return dir, proto, nil
}

func toIPNet(ipStr string, bits int) (net.IPNet, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return net.IPNet{}, fmt.Errorf("invalid IP: %s", ipStr)
	}

	totalBits := 32
	if ip.To4() == nil {
		totalBits = 128
	}

	mask := net.CIDRMask(bits, totalBits)
	return net.IPNet{
		IP:   ip.Mask(mask),
		Mask: mask,
	}, nil
}


func mapIpAndCidr(ipv4, ipv6 string, i4mask, i6mask int) ([]net.IPNet, error) {

	var ips []net.IPNet

	if i4mask == 0 {
		i4mask = 32
	}

	if i6mask == 0 {
		i6mask = 128
	}

	if ipv4 != "" {
		ipNet, err := toIPNet(ipv4, i4mask)
		if err != nil {
			return nil, err
		}
		ips = append(ips, ipNet)
	}

	if ipv6 != "" {
		ipNet, err := toIPNet(ipv6, i6mask)
		if err != nil {
			return nil, err
		}
		ips = append(ips, ipNet)
	}

	return ips, nil
}

func CreateFWFule(cfg config.Rule, ipv4, ipv6 string) (hcloud.FirewallRule, error) {

	dir, proto, err := mapDirAndProto(cfg.Direction, cfg.Protocol)
	if err != nil {
		return hcloud.FirewallRule{}, err
	}

	ips, err := mapIpAndCidr(ipv4, ipv6, cfg.IPv4mask, cfg.IPv6mask)

	if err != nil {
		return hcloud.FirewallRule{}, err
	}

	port := strconv.Itoa(cfg.Port)

	return hcloud.FirewallRule{
		Description: hcloud.Ptr(cfg.Description),
		Direction:   dir,
		Port:        hcloud.Ptr(port),
		Protocol:    proto,
		SourceIPs:   ips,
	}, nil
}

func UpdateFw(fwRules []hcloud.FirewallRule, fwId int64, token string, t int64) (*hcloud.Response, error) {

	client := hcloud.NewClient(hcloud.WithToken(token))
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t) * time.Second)
	defer cancel()

	actions, res, err := client.Firewall.SetRules(ctx, &hcloud.Firewall{ID: fwId}, hcloud.FirewallSetRulesOpts{
		Rules: fwRules,
	})

	if err != nil {
		return res, err
	}

	if err := client.Action.WaitFor(ctx, actions...); err != nil {
		return res, err
	}

	return res, nil
}
