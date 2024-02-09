package main

import (
	"encoding/json"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	netagents "github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/agents"
	"github.com/gophercloud/gophercloud/v2/pagination"
)

// NOTE: Clear installation of Zed on el9 starts giving unexpected time format, err:
//
//    parsing time "2023-03-16 18:35:47.845000+00:00": extra text: "+00:00"
//
// We had to replicate gophercloud code to use AnyTime instead the layout fixed in the original package.

// -*- modified neutron request -*-

func NeutronAgentList(c *gophercloud.ServiceClient, opts netagents.ListOptsBuilder) pagination.Pager {
	url := c.ServiceURL("agents")
	if opts != nil {
		query, err := opts.ToAgentListQuery()
		if err != nil {
			return pagination.Pager{Err: err}
		}
		url += query
	}
	return pagination.NewPager(c, url, func(r pagination.PageResult) pagination.Page {
		return NeutronAgentPage{pagination.LinkedPageBase{PageResult: r}}
	})
}

// -*- modified neurton response -*-

type NeutronAgent struct {
	netagents.Agent
}

// UnmarshalJSON helps to convert the timestamps into the time.Time type.
func (r *NeutronAgent) UnmarshalJSON(b []byte) error {
	type tmp netagents.Agent
	var s struct {
		tmp
		CreatedAt          AnyTime `json:"created_at"`
		StartedAt          AnyTime `json:"started_at"`
		HeartbeatTimestamp AnyTime `json:"heartbeat_timestamp"`
	}
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	r.Agent = netagents.Agent(s.tmp)

	r.CreatedAt = time.Time(s.CreatedAt)
	r.StartedAt = time.Time(s.StartedAt)
	r.HeartbeatTimestamp = time.Time(s.HeartbeatTimestamp)

	return nil
}

type NeutronAgentPage struct {
	pagination.LinkedPageBase
}

func (r NeutronAgentPage) NextPageURL() (string, error) {
	var s struct {
		Links []gophercloud.Link `json:"agents_links"`
	}
	err := r.ExtractInto(&s)
	if err != nil {
		return "", err
	}
	return gophercloud.ExtractNextURL(s.Links)
}

func (r NeutronAgentPage) IsEmpty() (bool, error) {
	if r.StatusCode == 204 {
		return true, nil
	}

	agents, err := ExtractNeutronAgents(r)
	return len(agents) == 0, err
}

func ExtractNeutronAgents(r pagination.Page) ([]NeutronAgent, error) {
	var s struct {
		Agents []NeutronAgent `json:"agents"`
	}
	err := (r.(NeutronAgentPage)).ExtractInto(&s)
	return s.Agents, err
}
