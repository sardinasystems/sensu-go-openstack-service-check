package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/baremetal/v1/conductors"
	volsrv "github.com/gophercloud/gophercloud/v2/openstack/blockstorage/extensions/services"
	cptsrv "github.com/gophercloud/gophercloud/v2/openstack/compute/v2/extensions/services"
	"github.com/gophercloud/gophercloud/v2/openstack/config"
	clouds "github.com/gophercloud/gophercloud/v2/openstack/config/clouds"
	sharesrv "github.com/gophercloud/gophercloud/v2/openstack/sharedfilesystems/v2/services"
	"github.com/jedib0t/go-pretty/v6/table"
	corev2 "github.com/sensu/core/v2"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Cloud                  string
	CloudsFile             string
	Service                string
	CriticalDisabledReason []string
	Debug                  bool
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
			Allow:     []string{"compute", "volume", "sharev2", "network", "orchestration", "container", "clustering", "baremetal"},
			Usage:     "Service to check",
			Value:     &plugin.Service,
		},
		&sensu.SlicePluginConfigOption[string]{
			Path:      "critical_disabled_reason",
			Argument:  "critical-reason",
			Shorthand: "r",
			Usage:     "Critical error from disabled reason (regexp)",
			Value:     &plugin.CriticalDisabledReason,
		},
		&sensu.PluginConfigOption[bool]{
			Argument:  "debug",
			Shorthand: "d",
			Usage:     "Debug API calls",
			Value:     &plugin.Debug,
		},
	}
)

func reasonMatch(reason string, regexps []string) bool {
	for _, pattern := range regexps {
		match, err := regexp.Match(pattern, []byte(reason))
		if err != nil {
			fmt.Printf("Error pattern regexp: %v\n", err)
			return false
		}

		if match {
			return match
		}
	}

	return false
}

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
	for _, pattern := range plugin.CriticalDisabledReason {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("Failed to compile regexp: %s: %w", pattern, err)
		}
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *corev2.Event) (int, error) {
	ctx, cf := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cf()

	// XXX clouds.WithLocations() hard to make conditional, as cloudOpts unexported and there no type suitable to make array
	if plugin.CloudsFile != "" {
		os.Setenv("OS_CLIENT_CONFIG_FILE", plugin.CloudsFile)
	}

	var httpCli *http.Client
	if plugin.Debug {
		httpCli = &http.Client{Transport: &http.Transport{}}
		// Wait till it support v2
		// httpCli = &http.Client{
		// 	Transport: &oscli.RoundTripper{
		// 		Rt:     &http.Transport{},
		// 		Logger: &oscli.DefaultLogger{},
		// 	},
		// }
	} else {
		httpCli = &http.Client{Transport: &http.Transport{}}
	}

	ao, eo, tlsCfg, err := clouds.Parse(clouds.WithCloudName(plugin.Cloud))
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	// check never need to reauth
	ao.AllowReauth = false

	pc, err := config.NewProviderClient(ctx, ao, config.WithHTTPClient(*httpCli), config.WithTLSConfig(tlsCfg))
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	switch plugin.Service {
	case "compute":
		return checkCompute(ctx, pc, eo)

	case "volume":
		return checkVolume(ctx, pc, eo)

	case "sharev2":
		return checkShare(ctx, pc, eo)

	case "network":
		return checkNetwork(ctx, pc, eo)

	case "orchestration":
		return checkOrchestration(ctx, pc, eo)

	case "container":
		return checkContainer(ctx, pc, eo)

	case "clustering":
		return checkClustering(ctx, pc, eo)

	case "baremetal":
		return checkBaremetal(ctx, pc, eo)

	default:
		return sensu.CheckStateUnknown, fmt.Errorf("unsupported service: %s", plugin.Service)
	}
}

func checkCompute(_ context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (int, error) {
	cli, err := openstack.NewComputeV2(pc, eo)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	pages, err := cptsrv.List(cli, nil).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := cptsrv.ExtractServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	sort.Slice(srvs, func(i, j int) bool {
		si, sj := srvs[i], srvs[j]
		return si.Binary < sj.Binary || (si.Binary == sj.Binary && si.Host < sj.Host)
	})

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Binary", "Host", "Zone", "Status", "State", "Updated At", "Disabled Reason"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.ID, srv.Binary, srv.Host, srv.Zone, srv.Status, srv.State, srv.UpdatedAt, srv.DisabledReason})

		if srv.Status == "enabled" && srv.State != "up" {
			ret = sensu.CheckStateCritical
		}

		if srv.Status == "disabled" && reasonMatch(srv.DisabledReason, plugin.CriticalDisabledReason) {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}

func checkVolume(_ context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (int, error) {
	cli, err := openstack.NewBlockStorageV3(pc, eo)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	pages, err := volsrv.List(cli, nil).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := volsrv.ExtractServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	sort.Slice(srvs, func(i, j int) bool {
		si, sj := srvs[i], srvs[j]
		return si.Binary < sj.Binary || (si.Binary == sj.Binary && si.Host < sj.Host)
	})

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Binary", "Host", "Zone", "Status", "State", "Updated At", "Disabled Reason"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.Binary, srv.Host, srv.Zone, srv.Status, srv.State, srv.UpdatedAt, srv.DisabledReason})

		if srv.Status == "enabled" && srv.State != "up" {
			ret = sensu.CheckStateCritical
		}

		if srv.Status == "disabled" && reasonMatch(srv.DisabledReason, plugin.CriticalDisabledReason) {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}

func checkShare(_ context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (int, error) {
	cli, err := openstack.NewSharedFileSystemV2(pc, eo)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	cli.Microversion = "2.7"

	pages, err := sharesrv.List(cli, nil).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := sharesrv.ExtractServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	sort.Slice(srvs, func(i, j int) bool {
		si, sj := srvs[i], srvs[j]
		return si.Binary < sj.Binary || (si.Binary == sj.Binary && si.Host < sj.Host)
	})

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

func checkNetwork(_ context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (int, error) {
	cli, err := openstack.NewNetworkV2(pc, eo)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

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

func checkOrchestration(_ context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (int, error) {
	cli, err := openstack.NewOrchestrationV1(pc, eo)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	pages, err := HeatServiceList(cli).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := ExtractHeatServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	sort.Slice(srvs, func(i, j int) bool {
		si, sj := srvs[i], srvs[j]
		return si.Binary < sj.Binary || (si.Binary == sj.Binary && si.Host < sj.Host)
	})

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Binary", "Host", "Status", "Report Interval", "Updated At"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.ID, srv.Binary, srv.Host, srv.Status, srv.ReportInterval, srv.UpdatedAt.As()})

		if srv.Status != "up" {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}

func checkContainer(_ context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (int, error) {
	cli, err := openstack.NewContainerV1(pc, eo)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	pages, err := ZunServiceList(cli).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := ExtractZunServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	sort.Slice(srvs, func(i, j int) bool {
		si, sj := srvs[i], srvs[j]
		return si.Binary < sj.Binary || (si.Binary == sj.Binary && si.Host < sj.Host)
	})

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Binary", "Host", "Availability Zone", "Disabled", "State", "Updated At", "Heartbeat", "Disable Reason"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.ID, srv.Binary, srv.Host, srv.AvailabilityZone, srv.Disabled, srv.State, srv.UpdatedAt.As(), srv.LastSeenUp.As(), srv.DisableReason})

		if !srv.Disabled && srv.State != "up" {
			ret = sensu.CheckStateCritical
		}

		if srv.Disabled && reasonMatch(srv.DisableReason, plugin.CriticalDisabledReason) {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}

func checkClustering(_ context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (int, error) {
	cli, err := openstack.NewClusteringV1(pc, eo)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}
	cli.Microversion = "1.7"

	pages, err := SenlinServiceList(cli).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := ExtractSenlinServices(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	sort.Slice(srvs, func(i, j int) bool {
		si, sj := srvs[i], srvs[j]
		return si.Binary < sj.Binary || (si.Binary == sj.Binary && si.Host < sj.Host)
	})

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Binary", "Host", "State", "Status", "Updated At", "Disable Reason"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.ID, srv.Binary, srv.Host, srv.State, srv.Status, srv.UpdatedAt.As(), srv.DisableReason})

		if srv.Status == "enabled" && srv.State != "up" {
			ret = sensu.CheckStateCritical
		}

		if srv.Status == "disabled" && reasonMatch(srv.DisableReason, plugin.CriticalDisabledReason) {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}

func checkBaremetal(_ context.Context, pc *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (int, error) {
	cli, err := openstack.NewBareMetalV1(pc, eo)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}
	cli.Microversion = "1.49"

	pages, err := conductors.List(cli, conductors.ListOpts{Detail: true}).AllPages()
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	srvs, err := conductors.ExtractConductors(pages)
	if err != nil {
		return sensu.CheckStateUnknown, err
	}

	sort.Slice(srvs, func(i, j int) bool {
		si, sj := srvs[i], srvs[j]
		return si.Hostname < sj.Hostname
	})

	ret := sensu.CheckStateOK

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Host", "Conductor Group", "Drivers", "Alive", "Updated At"})

	for _, srv := range srvs {
		t.AppendRow(table.Row{srv.Hostname, srv.ConductorGroup, strings.Join(srv.Drivers, " "), srv.Alive, srv.UpdatedAt})

		if !srv.Alive {
			ret = sensu.CheckStateCritical
		}
	}

	t.Render()

	return ret, nil
}
