package main

import (
	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/pagination"
)

type HeatService struct {
	Binary         string   `json:"binary"`
	ID             string   `json:"id"`
	EngineID       string   `json:"engine_id"`
	Host           string   `json:"host"`
	Hostname       string   `json:"hostname"`
	Status         string   `json:"status"`
	Topic          string   `json:"topic"`
	ReportInterval int      `json:"report_interval"`
	CreatedAt      AnyTime  `json:"created_at"`
	UpdatedAt      AnyTime  `json:"updated_at"`
	DeletedAt      *AnyTime `json:"deleted_at,omitempty"`
}

type HeatServicePage struct {
	pagination.SinglePageBase
}

func (page HeatServicePage) IsEmpty() (bool, error) {
	if page.StatusCode == 204 {
		return true, nil
	}

	services, err := ExtractHeatServices(page)
	return len(services) == 0, err
}

func ExtractHeatServices(r pagination.Page) ([]HeatService, error) {
	var s struct {
		Service []HeatService `json:"services"`
	}
	err := (r.(HeatServicePage)).ExtractInto(&s)
	return s.Service, err
}

func HeatServiceList(client *gophercloud.ServiceClient) pagination.Pager {
	url := client.ServiceURL("services")

	return pagination.NewPager(client, url, func(r pagination.PageResult) pagination.Page {
		return HeatServicePage{pagination.SinglePageBase(r)}
	})
}
