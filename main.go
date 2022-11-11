package main

import (
	"fmt"
	"os"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/services"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/jedib0t/go-pretty/v6/table"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Cloud      string
	CloudsFile string
	Service    string
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
			Allow:     []string{"compute", "network", "volume", "share"},
			Usage:     "Service to check",
			Value:     &plugin.Service,
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

	opts := &clientconfig.ClientOpts{
		Cloud: plugin.Cloud,
	}

	cli, err := clientconfig.NewServiceClient(plugin.Service, opts)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	switch plugin.Service {
	case "compute":
		return checkCompute(cli)

	default:
		return sensu.CheckStateUnknown, fmt.Errorf("unsupported service: %s", plugin.Service)
	}
}

func checkCompute(cli *gophercloud.ServiceClient) (int, error) {
	pages, err := services.List(cli, nil).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := services.ExtractServices(pages)
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
