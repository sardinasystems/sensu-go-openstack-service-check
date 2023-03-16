package main

import (
	"fmt"
	"net/http"
	"os"
	"sort"

	"github.com/gophercloud/gophercloud"
	volsrv "github.com/gophercloud/gophercloud/openstack/blockstorage/extensions/services"
	cptsrv "github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/services"
	sharesrv "github.com/gophercloud/gophercloud/openstack/sharedfilesystems/v2/services"
	oscli "github.com/gophercloud/utils/client"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/jedib0t/go-pretty/v6/table"
	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Cloud      string
	CloudsFile string
	Service    string
	Debug      bool
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-go-openstack-service-check",
			Short:    "Plugin to check OpenStack service states",
			Keyspace: "sensu.io/plugins/sensu-go-openstack-service-check/config",
		},
	}

	options = []sensu.ConfigOption{
		&sensu.PluginConfigOption[string]{
			Path:      "cloud",
			Env:       "OS_CLOUD",
			Argument:  "cloud",
			Shorthand: "c",
			Default:   "monitoring",
			Usage:     "Cloud used to access openstack API",
			Value:     &plugin.Cloud,
		},
		&sensu.PluginConfigOption[string]{
			Path:     "os_config_file",
			Env:      "OS_CLIENT_CONFIG_FILE",
			Argument: "os-config-file",
			Default:  "",
			Usage:    "Clouds.yaml file path",
			Value:    &plugin.CloudsFile,
		},
		&sensu.PluginConfigOption[string]{
			Path:      "service",
			Argument:  "service",
			Shorthand: "s",
			Default:   "compute",
			Allow:     []string{"compute", "volume", "sharev2", "network"},
			Usage:     "Service to check",
			Value:     &plugin.Service,
		},
		&sensu.PluginConfigOption[bool]{
			Argument:  "debug",
			Shorthand: "d",
			Usage:     "Debug API calls",
			Value:     &plugin.Debug,
		},
	}
)

func main() {
	useStdin := false
	fi, err := os.Stdin.Stat()
	if err != nil {
		fmt.Printf("Error check stdin: %v\n", err)
	}
	// Check the Mode bitmask for Named Pipe to indicate stdin is connected
	if fi.Mode()&os.ModeNamedPipe != 0 {
		useStdin = true
	}

	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, useStdin)
	check.Execute()
}

func checkArgs(event *corev2.Event) (int, error) {
	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {
	if plugin.CloudsFile != "" {
		os.Setenv("OS_CLIENT_CONFIG_FILE", plugin.CloudsFile)
	}

	var httpCli *http.Client
	if plugin.Debug {
		httpCli = &http.Client{
			Transport: &oscli.RoundTripper{
				Rt:     &http.Transport{},
				Logger: &oscli.DefaultLogger{},
			},
		}
	} else {
		httpCli = &http.Client{Transport: &http.Transport{}}
	}

	opts := &clientconfig.ClientOpts{
		Cloud:      plugin.Cloud,
		HTTPClient: httpCli,
	}

	cli, err := clientconfig.NewServiceClient(plugin.Service, opts)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	switch plugin.Service {
	case "compute":
		return checkCompute(cli)

	case "volume":
		return checkVolume(cli)

	case "sharev2":
		return checkShare(cli)

	case "network":
		return checkNetwork(cli)

	default:
		return sensu.CheckStateUnknown, fmt.Errorf("unsupported service: %s", plugin.Service)
	}
}

func checkCompute(cli *gophercloud.ServiceClient) (int, error) {
	pages, err := cptsrv.List(cli, nil).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := cptsrv.ExtractServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Binary", "Host", "Zone", "Status", "State", "Updated At"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.ID, srv.Binary, srv.Host, srv.Zone, srv.Status, srv.State, srv.UpdatedAt})

		if srv.Status == "enabled" && srv.State != "up" {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}

func checkVolume(cli *gophercloud.ServiceClient) (int, error) {
	pages, err := volsrv.List(cli, nil).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := volsrv.ExtractServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Binary", "Host", "Zone", "Status", "State", "Updated At"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.Binary, srv.Host, srv.Zone, srv.Status, srv.State, srv.UpdatedAt})

		if srv.Status == "enabled" && srv.State != "up" {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}

func checkShare(cli *gophercloud.ServiceClient) (int, error) {
	cli.Microversion = "2.7"

	pages, err := sharesrv.List(cli, nil).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := sharesrv.ExtractServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Binary", "Host", "Zone", "Status", "State", "Updated At"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.ID, srv.Binary, srv.Host, srv.Zone, srv.Status, srv.State, srv.UpdatedAt})

		if srv.Status == "enabled" && srv.State != "up" {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}

func checkNetwork(cli *gophercloud.ServiceClient) (int, error) {
	pages, err := NeutronAgentList(cli, nil).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, fmt.Errorf("List error: %w", err)
	}

	agents, err := ExtractNeutronAgents(pages)
	if err != nil {
		return sensu.CheckStateUnknown, fmt.Errorf("Unmarshal error: %w", err)
	}

	sort.Slice(agents, func(i, j int) bool {
		ai, aj := agents[i], agents[j]
		return ai.AgentType < aj.AgentType || (ai.AgentType == aj.AgentType && ai.Host < aj.Host)
	})

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Agent Type", "Host", "Availability Zone", "Alive", "State", "Binary", "Heartbeat"})

	for _, ag := range agents {
		t.AppendRow(table.Row{ag.ID, ag.AgentType, ag.Host, ag.AvailabilityZone, ag.Alive, ag.AdminStateUp, ag.Binary, ag.HeartbeatTimestamp})

		if ag.AdminStateUp && !ag.Alive {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}
