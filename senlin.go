package main

import (
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/pagination"
)

type SenlinService struct {
	Binary        string  `json:"binary"`
	DisableReason string  `json:"disable_reason"`
	Host          string  `json:"host"`
	ID            int     `json:"id"`
	State         string  `json:"state"`
	Status        string  `json:"status"`
	Topic         string  `json:"topic"`
	UpdatedAt     AnyTime `json:"updated_at"`
}

type SenlinServicePage struct {
	pagination.SinglePageBase
}

func (page SenlinServicePage) IsEmpty() (bool, error) {
	if page.StatusCode == 204 {
		return true, nil
	}

	services, err := ExtractSenlinServices(page)
	return len(services) == 0, err
}

func ExtractSenlinServices(r pagination.Page) ([]SenlinService, error) {
	var s struct {
		Services []SenlinService `json:"services"`
	}
	err := (r.(SenlinServicePage)).ExtractInto(&s)
	return s.Services, err
}

func SenlinServiceList(client *gophercloud.ServiceClient) pagination.Pager {
	url := client.ServiceURL("v1", "services")

	return pagination.NewPager(client, url, func(r pagination.PageResult) pagination.Page {
		return SenlinServicePage{pagination.SinglePageBase(r)}
	})
}
