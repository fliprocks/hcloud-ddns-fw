package hcloud_functions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fliprocks/hcloud-ddns-fw/internal/config"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

func mapRecordTypeAndIp(t, ipv4, ipv6 string) (hcloud.ZoneRRSetType, string, error) {
	switch strings.ToUpper(t) {
	case "A":
		return hcloud.ZoneRRSetTypeA, ipv4, nil
	case "AAAA":
		return hcloud.ZoneRRSetTypeAAAA, ipv6, nil
	default:
		return "", "", fmt.Errorf("Unsupported record type: %s", t)
	}
}

func SetRecord(z config.DNSZone, r config.Record, ipv4, ipv6, token string, t int64) (*hcloud.Response, error) {

	rrType, ip, err := mapRecordTypeAndIp(r.RecordType, ipv4, ipv6)
	if err != nil {
		return nil, err
	}

	client := hcloud.NewClient(hcloud.WithToken(token))
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t) * time.Second)
	defer cancel()

	action, res, err := client.Zone.SetRRSetRecords(ctx, &hcloud.ZoneRRSet{
		Zone: &hcloud.Zone{Name: z.Name},
		Name: r.Name,
		Type: rrType,
	}, hcloud.ZoneRRSetSetRecordsOpts{
		Records: []hcloud.ZoneRRSetRecord{
			{
				Value: ip,
				Comment: r.Comment,
			},
		},
	})

	if err != nil {
		return res, err
	}

	if err := client.Action.WaitFor(ctx, action); err != nil {
		return res, err
	}

	return res, nil
}

func SetTTL(z config.DNSZone, r config.Record, token string, t int64) (*hcloud.Response, error) {

	if r.TTL == 0 {
		r.TTL = 3600
	}

	client := hcloud.NewClient(hcloud.WithToken(token))
	
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t) * time.Second)
	defer cancel()

	action, res, err := client.Zone.ChangeRRSetTTL(ctx, &hcloud.ZoneRRSet{
		Zone: &hcloud.Zone{Name: z.Name},
		Name: r.Name,
		Type: hcloud.ZoneRRSetTypeA,
	}, hcloud.ZoneRRSetChangeTTLOpts{
		TTL: hcloud.Ptr(r.TTL),
	})
	if err != nil {
		return res, err
	}

	if err := client.Action.WaitFor(ctx, action); err != nil {
		return res, err
	}

	return res, nil
}