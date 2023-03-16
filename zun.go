package main

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

type ZunService struct {
	Binary           string  `json:"binary"`
	AvailabilityZone string  `json:"availability_zone"`
	State            string  `json:"state"`
	ReportCount      int     `json:"report_count"`
	Disabled         bool    `json:"disabled"`
	DisableReason    string  `json:"disable_reason"`
	ForceDown        bool    `json:"force_down"`
	Host             string  `json:"host"`
	ID               int     `json:"id"`
	LastSeenUp       AnyTime `json:"last_seen_up"`
	CreatedAt        AnyTime `json:"created_at"`
	UpdatedAt        AnyTime `json:"updated_at"`
}

type ZunServicePage struct {
	pagination.SinglePageBase
}

func (page ZunServicePage) IsEmpty() (bool, error) {
	if page.StatusCode == 204 {
		return true, nil
	}

	services, err := ExtractZunServices(page)
	return len(services) == 0, err
}

func ExtractZunServices(r pagination.Page) ([]ZunService, error) {
	var s struct {
		Service []ZunService `json:"services"`
	}
	err := (r.(ZunServicePage)).ExtractInto(&s)
	return s.Service, err
}

func ZunServiceList(client *gophercloud.ServiceClient) pagination.Pager {
	url := client.ServiceURL("services")

	return pagination.NewPager(client, url, func(r pagination.PageResult) pagination.Page {
		return ZunServicePage{pagination.SinglePageBase(r)}
	})
}
