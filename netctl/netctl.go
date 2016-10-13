package netctl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/codegangsta/cli"
	contivClient "github.com/contiv/contivmodel/client"
	"github.com/contiv/netplugin/version"
)

// DefaultMaster is the master to use when none is provided.
const DefaultMaster = "http://netmaster:9999"

func getClient(ctx *cli.Context) *contivClient.ContivClient {
	cl, err := contivClient.NewContivClient(ctx.GlobalString("netmaster"))
	if err != nil {
		errExit(ctx, 1, "Error connecting to netmaster", false)
	}

	return cl
}

func createPolicy(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Policy name required", true)
	}

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	errCheck(ctx, getClient(ctx).PolicyPost(&contivClient.Policy{
		PolicyName: policy,
		TenantName: tenant,
	}))

	fmt.Printf("Creating policy %s:%s\n", tenant, policy)
}

func deletePolicy(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Policy name required", true)
	}

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	fmt.Printf("Deleting policy %s:%s\n", tenant, policy)

	errCheck(ctx, getClient(ctx).PolicyDelete(tenant, policy))
}

func inspectPolicy(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Policy name required", true)
	}

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	fmt.Printf("Inspecting policy: %s tenant: %s\n", policy, tenant)

	pol, err := getClient(ctx).PolicyInspect(tenant, policy)
	errCheck(ctx, err)

	content, err := json.MarshalIndent(pol, "", "  ")
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func listPolicies(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	tenant := ctx.String("tenant")

	policies, err := getClient(ctx).PolicyList()
	errCheck(ctx, err)

	var filtered []*contivClient.Policy

	for _, policy := range *policies {
		if policy.TenantName == tenant || ctx.Bool("all") {
			filtered = append(filtered, policy)
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, filtered)
	} else if ctx.Bool("quiet") {
		policies := ""
		for _, policy := range filtered {
			policies += policy.PolicyName + "\n"
		}
		os.Stdout.WriteString(policies)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Tenant\tPolicy\n"))
		writer.Write([]byte("------\t------\n"))

		for _, policy := range filtered {
			writer.Write([]byte(fmt.Sprintf("%s\t%s\n", policy.TenantName, policy.PolicyName)))
		}
	}
}

func addRule(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, exitHelp, "Policy name and Rule ID required", true)
	}

	dir := ctx.String("direction")
	if dir == "in" {
		if ctx.String("to-group") != "" {
			errExit(ctx, exitHelp, "Cant specify to-group for incoming rule", false)
		}
		if ctx.String("to-network") != "" {
			errExit(ctx, exitHelp, "Cant specify to-network for incoming rule", false)
		}
		if ctx.String("to-ip-address") != "" {
			errExit(ctx, exitHelp, "Cant specify to-ip-address for incoming rule", false)
		}

		// If from EPG is specified, make sure from network is specified too
		if ctx.String("from-group") != "" && ctx.String("from-network") != "" {
			errExit(ctx, exitHelp, "Can't specify both from-group argument and -from-network ", false)
		}
	} else if dir == "out" {
		if ctx.String("from-group") != "" {
			errExit(ctx, exitHelp, "Cant specify from-group for outgoing rule", false)
		}
		if ctx.String("from-network") != "" {
			errExit(ctx, exitHelp, "Cant specify from-network for outgoing rule", false)
		}
		if ctx.String("from-ip-address") != "" {
			errExit(ctx, exitHelp, "Cant specify from-ip-address for outgoing rule", false)
		}

		// If to EPG is specified, make sure to network is specified too
		if ctx.String("to-group") != "" && ctx.String("to-network") != "" {
			errExit(ctx, exitHelp, "Can't specify both -to-group and -to-network", false)
		}
	} else {
		errExit(ctx, exitHelp, "Unknown direction", false)
	}

	errCheck(ctx, getClient(ctx).RulePost(&contivClient.Rule{
		TenantName:        ctx.String("tenant"),
		PolicyName:        ctx.Args()[0],
		RuleID:            ctx.Args()[1],
		Priority:          ctx.Int("priority"),
		Direction:         ctx.String("direction"),
		FromEndpointGroup: ctx.String("from-group"),
		ToEndpointGroup:   ctx.String("to-group"),
		FromNetwork:       ctx.String("from-network"),
		ToNetwork:         ctx.String("to-network"),
		FromIpAddress:     ctx.String("from-ip-address"),
		ToIpAddress:       ctx.String("to-ip-address"),
		Protocol:          ctx.String("protocol"),
		Port:              ctx.Int("port"),
		Action:            ctx.String("action"),
	}))
}

func deleteRule(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, exitHelp, "Policy name and Rule ID required", true)
	}

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]
	ruleID := ctx.Args()[1]

	errCheck(ctx, getClient(ctx).RuleDelete(tenant, policy, ruleID))
}

func listRules(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Policy name required", true)
	}

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	rules, err := getClient(ctx).RuleList()
	errCheck(ctx, err)

	writeRules := map[int][]*contivClient.Rule{}

	var writePrio []int
	var results []*contivClient.Rule

	for _, rule := range *rules {

		if rule.TenantName == tenant && rule.PolicyName == policy {
			prio := rule.Priority

			if _, ok := writeRules[prio]; !ok {
				writeRules[prio] = make([]*contivClient.Rule, 0)
				writePrio = append(writePrio, prio)
			}

			writeRules[prio] = append(writeRules[prio], rule)
		}
	}

	sort.Ints(writePrio)

	for _, prio := range writePrio {
		for _, rule := range writeRules[prio] {
			results = append(results, rule)
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, results)
	} else if ctx.Bool("quiet") {
		rules := ""
		for _, rule := range results {
			rules += rule.RuleID + "\n"
		}
		os.Stdout.WriteString(rules)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Incoming Rules:\n"))
		writer.Write([]byte("Rule\tPriority\tFrom EndpointGroup\tFrom Network\tFrom IpAddress\tProtocol\tPort\tAction\n"))
		writer.Write([]byte("----\t--------\t------------------\t------------\t---------\t--------\t----\t------\n"))

		for _, rule := range results {
			if rule.Direction == "in" {
				writer.Write([]byte(fmt.Sprintf(
					"%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
					rule.RuleID,
					rule.Priority,
					rule.FromEndpointGroup,
					rule.FromNetwork,
					rule.FromIpAddress,
					rule.Protocol,
					rule.Port,
					rule.Action,
				)))
			}
		}

		writer.Write([]byte("Outgoing Rules:\n"))
		writer.Write([]byte("Rule\tPriority\tTo EndpointGroup\tTo Network\tTo IpAddress\tProtocol\tPort\tAction\n"))
		writer.Write([]byte("----\t--------\t----------------\t----------\t---------\t--------\t----\t------\n"))

		for _, rule := range results {
			if rule.Direction == "out" {
				writer.Write([]byte(fmt.Sprintf(
					"%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
					rule.RuleID,
					rule.Priority,
					rule.ToEndpointGroup,
					rule.ToNetwork,
					rule.ToIpAddress,
					rule.Protocol,
					rule.Port,
					rule.Action,
				)))
			}
		}
	}
}

func createNetProfile(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Net profile name required", true)
	}

	burst := ctx.Int("burst")
	bandwidth := ctx.String("bandwidth")
	dscp := ctx.Int("dscp")
	tenant := ctx.String("tenant")

	name := ctx.Args()[0]

	errCheck(ctx, getClient(ctx).NetprofilePost(&contivClient.Netprofile{
		Burst:       burst,
		Bandwidth:   bandwidth,
		DSCP:        dscp,
		ProfileName: name,
		TenantName:  tenant,
	}))
	fmt.Printf("Creating netprofile %s:%s\n", tenant, name)
}

func deleteNetProfile(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Net profile name required", true)
	}

	name := ctx.Args()[0]
	tenant := ctx.String("tenant")

	errCheck(ctx, getClient(ctx).NetprofileDelete(tenant, name))

}

func listNetProfiles(ctx *cli.Context) {
	var (
		bandwidth string
		bw        []string
	)
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	tenant := ctx.String("tenant")

	profileList, err := getClient(ctx).NetprofileList()
	errCheck(ctx, err)

	var filtered []*contivClient.Netprofile

	for _, profile := range *profileList {
		if profile.TenantName == tenant || ctx.Bool("all") {
			filtered = append(filtered, profile)
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, filtered)
	} else if ctx.Bool("quiet") {
		profiles := ""
		for _, profile := range filtered {
			profiles += profile.ProfileName + "\n"
		}
		os.Stdout.WriteString(profiles)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Name\tTenant\tBandwidth\tDSCP\tburst size\n"))
		writer.Write([]byte("------\t------\t---------\t--------\t----------\n"))

		for _, netProfile := range filtered {
			if netProfile.Bandwidth != "" {
				regex := regexp.MustCompile("[0-9]+")
				bw = regex.FindAllString(netProfile.Bandwidth, -1)
				if strings.ContainsAny(netProfile.Bandwidth, "g|G") {
					bandwidth = "Gbps"
				} else if strings.ContainsAny(netProfile.Bandwidth, "m|M") {
					bandwidth = "Mbps"
				} else if strings.ContainsAny(netProfile.Bandwidth, "k|K") {
					bandwidth = "Kbps"
				}
				npBandwidth := bw[0] + bandwidth
				writer.Write(
					[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\t%v\n",
						netProfile.ProfileName,
						netProfile.TenantName,
						npBandwidth,
						netProfile.DSCP,
						netProfile.Burst,
					)))
			} else {
				writer.Write(
					[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\n",
						netProfile.ProfileName,
						netProfile.TenantName,
						netProfile.Bandwidth,
						netProfile.DSCP,
					)))
			}

		}
	}
}

func inspectNetprofile(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Net profile name required", true)
	}

	tenant := ctx.String("tenant")
	netprofile := ctx.Args()[0]
	fmt.Printf("Inspecting netprofile:%s for %s", netprofile, tenant)

	profileList, err := getClient(ctx).NetprofileInspect(tenant, netprofile)
	errCheck(ctx, err)

	content, err := json.MarshalIndent(profileList, "", "  ")
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}

	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")

}

func createNetwork(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Network name required", true)
	}

	subnet := ctx.String("subnet")
	gateway := ctx.String("gateway")

	subnetv6 := ctx.String("subnetv6")
	gatewayv6 := ctx.String("gatewayv6")

	if subnet == "" {
		errExit(ctx, exitHelp, "Subnet is required", true)
	}
	if gateway != "" {
		if ok := net.ParseIP(gateway); ok == nil {
			errExit(ctx, exitHelp, "Invalid gateway - Enter in A.B.C.D format", true)
		}
	}
	if gatewayv6 != "" {
		if ok := net.ParseIP(gatewayv6); ok == nil {
			errExit(ctx, exitHelp, "Invalid IPv6 gateway ", true)
		}
	}

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]
	encap := ctx.String("encap")
	pktTag := ctx.Int("pkt-tag")
	nwType := ctx.String("nw-type")

	errCheck(ctx, getClient(ctx).NetworkPost(&contivClient.Network{
		TenantName:  tenant,
		NetworkName: network,
		Encap:       encap,
		Subnet:      subnet,
		Gateway:     gateway,
		Ipv6Subnet:  subnetv6,
		Ipv6Gateway: gatewayv6,
		PktTag:      pktTag,
		NwType:      nwType,
	}))

	fmt.Printf("Creating network %s:%s\n", tenant, network)
}

func deleteNetwork(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Network name required", true)
	}

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]

	fmt.Printf("Deleting network %s:%s\n", tenant, network)

	errCheck(ctx, getClient(ctx).NetworkDelete(tenant, network))

}

func inspectNetwork(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Network name required", true)
	}

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]

	fmt.Printf("Inspecting network: %s tenant: %s\n", network, tenant)

	net, err := getClient(ctx).NetworkInspect(tenant, network)
	errCheck(ctx, err)

	content, err := json.MarshalIndent(net, "", "  ")
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func listNetworks(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	tenant := ctx.String("tenant")

	netList, err := getClient(ctx).NetworkList()
	errCheck(ctx, err)

	var filtered []*contivClient.Network

	if ctx.Bool("all") {
		filtered = *netList
	} else {
		for _, net := range *netList {
			if net.TenantName == tenant {
				filtered = append(filtered, net)
			}
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, &filtered)
	} else if ctx.Bool("quiet") {
		networks := ""
		for _, network := range filtered {
			networks += network.NetworkName + "\n"
		}
		os.Stdout.WriteString(networks)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Tenant\tNetwork\tNw Type\tEncap type\tPacket tag\tSubnet\tGateway\tIPv6Subnet\tIPv6Gateway\n"))
		writer.Write([]byte("------\t-------\t-------\t----------\t----------\t-------\t------\t----------\t-----------\n"))

		for _, net := range filtered {
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
					net.TenantName,
					net.NetworkName,
					net.NwType,
					net.Encap,
					net.PktTag,
					net.Subnet,
					net.Gateway,
					net.Ipv6Subnet,
					net.Ipv6Gateway,
				)))
		}
	}
}

func createTenant(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Tenant name required", true)
	}

	tenant := ctx.Args()[0]

	errCheck(ctx, getClient(ctx).TenantPost(&contivClient.Tenant{
		TenantName: tenant,
	}))

	fmt.Printf("Creating tenant: %s\n", tenant)
}

func deleteTenant(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Tenant name required", true)
	}

	tenant := ctx.Args()[0]

	fmt.Printf("Deleting tenant %s\n", tenant)

	errCheck(ctx, getClient(ctx).TenantDelete(tenant))
}

func inspectTenant(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Tenant name required", true)
	}

	tenant := ctx.Args()[0]

	fmt.Printf("Inspecting tenant: %s  ", tenant)

	ten, err := getClient(ctx).TenantInspect(tenant)
	errCheck(ctx, err)

	content, err := json.MarshalIndent(ten, "", "  ")
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func listTenants(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	tenantList, err := getClient(ctx).TenantList()
	errCheck(ctx, err)

	if ctx.Bool("json") {
		dumpJSONList(ctx, tenantList)
	} else if ctx.Bool("quiet") {
		tenants := ""
		for _, tenant := range *tenantList {
			tenants += tenant.TenantName + "\n"
		}
		os.Stdout.WriteString(tenants)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Name\t\n"))
		writer.Write([]byte("------\t\n"))

		for _, tenant := range *tenantList {
			writer.Write(
				[]byte(fmt.Sprintf("%v\t\n",
					tenant.TenantName,
				)))
		}
	}
}

func inspectEndpoint(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Endpoint name required", true)
	}

	epid := ctx.Args()[0]

	fmt.Printf("Inspecting endpoint: %s\n", epid)

	net, err := getClient(ctx).EndpointInspect(epid)
	errCheck(ctx, err)

	content, err := json.MarshalIndent(net, "", "  ")
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func createEndpointGroup(ctx *cli.Context) {
	if len(ctx.Args()) != 2 {
		errExit(ctx, exitHelp, "Network and group name required", true)
	}

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]
	group := ctx.Args()[1]
	netprofile := ctx.String("networkprofile")

	policies := ctx.StringSlice("policy")

	extContractsGrps := ctx.StringSlice("external-contract")
	errCheck(ctx, getClient(ctx).EndpointGroupPost(&contivClient.EndpointGroup{
		TenantName:       tenant,
		NetworkName:      network,
		GroupName:        group,
		NetProfile:       netprofile,
		Policies:         policies,
		ExtContractsGrps: extContractsGrps,
	}))

	fmt.Printf("Creating EndpointGroup %s:%s\n", tenant, group)
}

func inspectEndpointGroup(ctx *cli.Context) {

	tenant := ctx.String("tenant")
	endpointGroup := ctx.Args()[0]

	fmt.Printf("Inspecting endpointGroup: %s tenant: %s\n", endpointGroup, tenant)

	epg, err := getClient(ctx).EndpointGroupInspect(tenant, endpointGroup)
	errCheck(ctx, err)

	content, err := json.MarshalIndent(epg, "", "  ")
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func deleteEndpointGroup(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Endpoint name required", true)
	}

	tenant := ctx.String("tenant")
	group := ctx.Args()[0]

	errCheck(ctx, getClient(ctx).EndpointGroupDelete(tenant, group))
}

func listEndpointGroups(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	tenant := ctx.String("tenant")

	epgList, err := getClient(ctx).EndpointGroupList()
	errCheck(ctx, err)

	filtered := []*contivClient.EndpointGroup{}

	for _, group := range *epgList {
		if group.TenantName == tenant || ctx.Bool("all") {
			filtered = append(filtered, group)
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, filtered)
	} else if ctx.Bool("quiet") {
		epgs := ""
		for _, epg := range filtered {
			epgs += epg.GroupName + "\n"
		}
		os.Stdout.WriteString(epgs)
	} else {

		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Tenant\tGroup\tNetwork\tPolicies\tNetwork profile\n"))
		writer.Write([]byte("------\t-----\t-------\t--------\t---------------\n"))
		for _, group := range filtered {
			policies := ""
			if group.Policies != nil {
				policyList := []string{}
				for _, p := range group.Policies {
					policyList = append(policyList, p)
				}
				policies = strings.Join(policyList, ",")
			}
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\t%v\n",
					group.TenantName,
					group.GroupName,
					group.NetworkName,
					policies,
					group.NetProfile,
				)))
		}
	}
}

//addBgp is a netctl interface routine to add
//bgp config
func addBgp(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Host name required", true)
	}

	hostname := ctx.Args()[0]
	routerip := ctx.String("router-ip")
	asid := ctx.String("as")
	neighboras := ctx.String("neighbor-as")
	neighbor := ctx.String("neighbor")

	//Error checks
	_, _, err := net.ParseCIDR(routerip)
	if err != nil {
		errExit(ctx, exitHelp, "Wrong CIDR format. Enter in x.x.x.x/len format", true)
	}

	ip := net.ParseIP(neighbor)
	if ip == nil {
		errExit(ctx, exitHelp, "Wrong IP format. Enter in x.x.x.x format", true)
	}

	if routerip == "" || asid == "" || neighbor == "" || neighboras == "" {
		errExit(ctx, exitHelp, "Missing attributes", true)
	}

	errCheck(ctx, getClient(ctx).BgpPost(&contivClient.Bgp{
		As:         asid,
		Hostname:   hostname,
		Neighbor:   neighbor,
		NeighborAs: neighboras,
		Routerip:   routerip,
	}))

}

//deleteBgp is a netctl interface routine to delete
//bgp config
func deleteBgp(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Host name required", true)
	}

	hostname := ctx.Args()[0]
	fmt.Printf("Deleting Bgp router config: %s\n", hostname)

	errCheck(ctx, getClient(ctx).BgpDelete(hostname))
}

//listBgpNeighbors is netctl interface routine to list
//Bgp neighbor configs for a given host
func listBgp(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	bgpList, err := getClient(ctx).BgpList()
	errCheck(ctx, err)

	if ctx.Bool("json") {
		dumpJSONList(ctx, bgpList)
	} else if ctx.Bool("quite") {
		bgpName := ""
		for _, bgp := range *bgpList {
			bgpName += bgp.Hostname + "\n"
		}
		os.Stdout.WriteString(bgpName)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("HostName\tRouterIP\tAS\tNeighbor\tNeighborAS\n"))
		writer.Write([]byte("---------\t--------\t-------\t--------\t-------\n"))
		for _, group := range *bgpList {
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\t%v\n",
					group.Hostname,
					group.Routerip,
					group.As,
					group.Neighbor,
					group.NeighborAs,
				)))
		}
	}
}

func inspectBgp(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Host name required", true)
	}

	hostname := ctx.Args()[0]

	fmt.Printf("netctl. Inspecting bgp: %s\n", hostname)

	bgp, err := getClient(ctx).BgpInspect(hostname)
	errCheck(ctx, err)

	content, err := json.MarshalIndent(bgp, "", "  ")
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func showGlobal(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	list, err := getClient(ctx).GlobalList()
	errCheck(ctx, err)

	if ctx.Bool("json") {
		dumpJSONList(ctx, list)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		defer writer.Flush()
		for _, gl := range *list {
			writer.Write([]byte(fmt.Sprintf("Fabric mode: %v\n", gl.NetworkInfraType)))
			writer.Write([]byte(fmt.Sprintf("Forward mode: %v\n", gl.FwdMode)))
			writer.Write([]byte(fmt.Sprintf("Vlan Range: %v\n", gl.Vlans)))
			writer.Write([]byte(fmt.Sprintf("Vxlan range: %v\n", gl.Vxlans)))
		}
	}
}

func inspectGlobal(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	fmt.Printf("Inspecting global\n")

	ginfo, err := getClient(ctx).GlobalInspect("global")
	errCheck(ctx, err)

	content, err := json.MarshalIndent(ginfo, "", "  ")
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func setGlobal(ctx *cli.Context) {
	fabMode := ctx.String("fabric-mode")
	vlans := ctx.String("vlan-range")
	vxlans := ctx.String("vxlan-range")
	fwdMode := ctx.String("fwd-mode")

	global, _ := getClient(ctx).GlobalGet("global")

	if fabMode != "" {
		global.NetworkInfraType = fabMode
	}
	if vlans != "" {
		global.Vlans = vlans
	}

	if vxlans != "" {
		global.Vxlans = vxlans
	}
	if fwdMode != "" {
		global.FwdMode = fwdMode
	}

	errCheck(ctx, getClient(ctx).GlobalPost(global))
}

func dumpJSONList(ctx *cli.Context, list interface{}) {
	content, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func dumpInspectList(ctx *cli.Context, list interface{}) {
	content, err := json.MarshalIndent(list, "", "  ")
	newContent := bytes.Split(content, []byte("link-sets"))
	fmt.Println(newContent)
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	//	os.Stdout.Write(newContent[0])
	fmt.Printf("%s", newContent[0])
	os.Stdout.WriteString("\n")
}

func showVersion(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	ver := version.Info{}
	if err := getObject(ctx, versionURL(ctx), &ver); err != nil {
		fmt.Printf("Unable to fetch version information\n")
	} else {
		fmt.Printf("Client Version:\n")
		fmt.Printf(version.String())
		fmt.Printf("\n")
		fmt.Printf("Server Version:\n")
		fmt.Printf(version.StringFromInfo(&ver))
	}
}

func createAppProfile(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Profile name required", true)
	}

	tenant := ctx.String("tenant")
	prof := ctx.Args()[0]

	groups := strings.Split(ctx.String("group"), ",")
	if ctx.String("group") == "" {
		groups = []string{}
	}

	errCheck(ctx, getClient(ctx).AppProfilePost(&contivClient.AppProfile{
		TenantName:     tenant,
		AppProfileName: prof,
		EndpointGroups: groups,
	}))

	fmt.Printf("Creating AppProfile %s:%s\n", tenant, prof)
}

func updateAppProfile(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Profile name required", true)
	}

	tenant := ctx.String("tenant")
	prof := ctx.Args()[0]

	groups := strings.Split(ctx.String("group"), ",")
	if ctx.String("group") == "" {
		groups = []string{}
	}

	errCheck(ctx, getClient(ctx).AppProfilePost(&contivClient.AppProfile{
		TenantName:     tenant,
		AppProfileName: prof,
		EndpointGroups: groups,
	}))
}

func deleteAppProfile(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Profile name required", true)
	}

	tenant := ctx.String("tenant")
	prof := ctx.Args()[0]

	errCheck(ctx, getClient(ctx).AppProfileDelete(tenant, prof))
}

func listAppProfiles(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	tenant := ctx.String("tenant")

	profList, err := getClient(ctx).AppProfileList()
	errCheck(ctx, err)

	filtered := []*contivClient.AppProfile{}

	for _, prof := range *profList {
		if prof.TenantName == tenant || ctx.Bool("all") {
			filtered = append(filtered, prof)
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, filtered)
	} else if ctx.Bool("quiet") {
		profiles := ""
		for _, p := range filtered {
			profiles += p.AppProfileName + "\n"
		}
		os.Stdout.WriteString(profiles)
	} else {

		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Tenant\tAppProfile\tGroups\n"))
		writer.Write([]byte("------\t----------\t------\n"))
		for _, p := range filtered {
			groups := ""
			if p.EndpointGroups != nil {
				groupList := []string{}
				for _, epg := range p.EndpointGroups {
					groupList = append(groupList, epg)
				}
				groups = strings.Join(groupList, ",")
			}
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\n",
					p.TenantName,
					p.AppProfileName,
					groups,
				)))
		}
	}
}

func listAppProfEpgs(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Profile name required", true)
	}

	tenant := ctx.String("tenant")
	prof := ctx.Args()[0]

	p, err := getClient(ctx).AppProfileGet(tenant, prof)
	errCheck(ctx, err)
	if ctx.Bool("json") {
		dumpJSONList(ctx, p)
	} else {
		groups := ""
		if p.EndpointGroups != nil {
			groupList := []string{}
			for _, epg := range p.EndpointGroups {
				groupList = append(groupList, epg)
			}
			groups = strings.Join(groupList, ",")
		}
		os.Stdout.WriteString(groups)
	}
}

//createServiceLB is a netctl interface routine to delete
//service object
func createServiceLB(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Service name required", true)
	}

	serviceName := ctx.Args()[0]
	serviceSubnet := ctx.String("network")
	tenantName := ctx.String("tenant")
	if len(tenantName) == 0 {
		tenantName = "default"
	}
	selectors := ctx.StringSlice("selector")
	ports := ctx.StringSlice("port")
	ipAddress := ctx.String("preferred-ip")
	service := &contivClient.ServiceLB{
		ServiceName: serviceName,
		TenantName:  tenantName,
		NetworkName: serviceSubnet,
		IpAddress:   ipAddress,
	}
	service.Selectors = append(service.Selectors, selectors...)
	service.Ports = append(service.Ports, ports...)
	errCheck(ctx, getClient(ctx).ServiceLBPost(service))

	fmt.Printf("Creating ServiceLB %s:%s\n", tenantName, serviceName)
}

//deleteServiceLB is a netctl interface routine to delete
//service object
func deleteServiceLB(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Service name required", true)
	}

	serviceName := ctx.Args()[0]
	tenantName := ctx.String("tenant")
	if len(tenantName) == 0 {
		tenantName = "default"
	}
	fmt.Printf("Deleting Service  %s,%s\n", serviceName, tenantName)

	errCheck(ctx, getClient(ctx).ServiceLBDelete(tenantName, serviceName))
}

//listServiceLB is a netctl interface routine to list
//service object
func listServiceLB(ctx *cli.Context) {

	tenantName := ctx.String("tenant")
	if len(tenantName) == 0 {
		tenantName = "default"
	}
	svcList, err := getClient(ctx).ServiceLBList()
	errCheck(ctx, err)

	filtered := []*contivClient.ServiceLB{}

	if ctx.Bool("all") {
		filtered = *svcList
	} else {
		for _, svc := range *svcList {
			if svc.TenantName == tenantName {
				filtered = append(filtered, svc)
			}
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, filtered)
	} else if ctx.Bool("quiet") {
		services := ""
		for _, service := range filtered {
			services += service.ServiceName + "\n"
		}
		os.Stdout.WriteString(services)
	} else {

		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("ServiceName\tTenant\tNetwork\tSelectors\n"))
		writer.Write([]byte("---------\t--------\t-------\t-------\n"))
		for _, group := range filtered {
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\t\n",
					group.ServiceName,
					group.TenantName,
					group.NetworkName,
					group.Selectors,
				)))
		}
	}
}

func listExternalContracts(ctx *cli.Context) {
	if len(ctx.Args()) != 0 {
		errExit(ctx, exitHelp, "More arguments than required", true)
	}

	extContractsGroupsList, err := getClient(ctx).ExtContractsGroupList()
	errCheck(ctx, err)

	tenant := ctx.String("tenant")

	var filtered []*contivClient.ExtContractsGroup

	for _, extContractsGroup := range *extContractsGroupsList {
		if extContractsGroup.TenantName == tenant || ctx.Bool("all") {
			filtered = append(filtered, extContractsGroup)
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, filtered)
	} else if ctx.Bool("quiet") {
		contractsGroupNames := ""
		for _, extContractsGroup := range filtered {
			contractsGroupNames += extContractsGroup.ContractsGroupName + "\n"
		}
		os.Stdout.WriteString(contractsGroupNames)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()

		writer.Write([]byte("Tenant\tName\t\tType\t\tContracts\n"))
		writer.Write([]byte("------\t------\t\t------\t\t-------\n"))
		for _, extContracts := range filtered {

			writer.Write([]byte(fmt.Sprintf("%s\t%s\t\t%s\t%s\n", extContracts.TenantName, extContracts.ContractsGroupName, extContracts.ContractsType, extContracts.Contracts)))

		}
	}
}

func deleteExternalContracts(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Contracts group name required", true)
	}

	contractsGroupName := ctx.Args()[0]
	tenant := ctx.String("tenant")

	fmt.Printf("Deleting external contracts group %s in tenant %s\n", contractsGroupName, tenant)
	errCheck(ctx, getClient(ctx).ExtContractsGroupDelete(tenant, contractsGroupName))

}

func createExternalContracts(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Contracts group name required", true)
	}

	var contractsType string
	if ctx.Bool("provided") && ctx.Bool("consumed") {
		errExit(ctx, exitHelp, "Cannot use both provided and consumed", false)
	} else if ctx.Bool("provided") {
		contractsType = "provided"
	} else if ctx.Bool("consumed") {
		contractsType = "consumed"
	} else {
		errExit(ctx, exitHelp, "Either provided or consumed must be specified", false)
	}

	tenant := ctx.String("tenant")

	contracts := ctx.StringSlice("contract")
	if len(contracts) == 0 {
		errExit(ctx, exitHelp, "Contracts not provided", false)
	}

	contractsGroupName := ctx.Args()[0]

	errCheck(ctx, getClient(ctx).ExtContractsGroupPost(&contivClient.ExtContractsGroup{
		TenantName:         tenant,
		ContractsGroupName: contractsGroupName,
		ContractsType:      contractsType,
		Contracts:          contracts,
	}))

	fmt.Printf("Creating ExternalContracts %s:%s\n", tenant, contractsGroupName)
}

func inspectServiceLb(ctx *cli.Context) {
	if len(ctx.Args()) != 1 {
		errExit(ctx, exitHelp, "Service group name required", true)
	}

	tenant := ctx.String("tenant")
	service := ctx.Args()[0]

	fmt.Printf("Inspecting service: %s tenant: %s\n", service, tenant)

	net, err := getClient(ctx).ServiceLBInspect(tenant, service)
	errCheck(ctx, err)

	content, err := json.MarshalIndent(net, "", "  ")
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}
