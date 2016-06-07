package netctl

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Sirupsen/logrus"
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
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	logrus.Infof("Creating policy %s:%s", tenant, policy)

	errCheck(ctx, getClient(ctx).PolicyPost(&contivClient.Policy{
		PolicyName: policy,
		TenantName: tenant,
	}))
}

func deletePolicy(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	logrus.Infof("Deleting policy %s:%s", tenant, policy)

	errCheck(ctx, getClient(ctx).PolicyDelete(tenant, policy))
}

func listPolicies(ctx *cli.Context) {
	argCheck(0, ctx)

	tenant := ctx.String("tenant")

	policies, err := getClient(ctx).PolicyList()
	errCheck(ctx, err)

	var filtered []*contivClient.Policy

	if !ctx.Bool("all") {
		for _, policy := range *policies {
			if policy.TenantName == tenant {
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
}

func addRule(ctx *cli.Context) {
	argCheck(2, ctx)

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
		if ctx.String("from-group") != "" && ctx.String("from-network") == "" {
			errExit(ctx, exitHelp, "from-group argument requires -from-network too", false)
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
		if ctx.String("to-group") != "" && ctx.String("to-network") == "" {
			errExit(ctx, exitHelp, "-to-group argument requires -to-network too", false)
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
	argCheck(2, ctx)

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]
	ruleID := ctx.Args()[1]

	errCheck(ctx, getClient(ctx).RuleDelete(tenant, policy, ruleID))
}

func listRules(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	rules, err := getClient(ctx).RuleList()
	errCheck(ctx, err)

	writeRules := map[int][]*contivClient.Rule{}

	var writePrio []int

	for _, rule := range *rules {
		if ctx.Bool("all") || (rule.TenantName == tenant && rule.PolicyName == policy) {
			prio := rule.Priority

			if _, ok := writeRules[prio]; !ok {
				writeRules[prio] = make([]*contivClient.Rule, 0)
				writePrio = append(writePrio, prio)
			}

			writeRules[prio] = append(writeRules[prio], rule)
		}
	}

	sort.Ints(writePrio)

	results := []*contivClient.Rule{}

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

func createNetwork(ctx *cli.Context) {
	argCheck(1, ctx)

	subnet := ctx.String("subnet")
	gateway := ctx.String("gateway")

	if subnet == "" {
		errExit(ctx, exitHelp, "Subnet is required", true)
	}
	if gateway != "" {
		if ok := net.ParseIP(gateway); ok == nil {
			errExit(ctx, exitHelp, "Invalid gateway - Enter in A.B.C.D format", true)
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
		PktTag:      pktTag,
		NwType:      nwType,
	}))
}

func deleteNetwork(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]

	logrus.Infof("Deleting network %s:%s", tenant, network)

	errCheck(ctx, getClient(ctx).NetworkDelete(tenant, network))

}

func listNetworks(ctx *cli.Context) {
	argCheck(0, ctx)

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
		writer.Write([]byte("Tenant\tNetwork\tNw Type\tEncap type\tPacket tag\tSubnet\tGateway\n"))
		writer.Write([]byte("------\t-------\t-------\t----------\t----------\t-------\t------\n"))

		for _, net := range filtered {
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
					net.TenantName,
					net.NetworkName,
					net.NwType,
					net.Encap,
					net.PktTag,
					net.Subnet,
					net.Gateway,
				)))
		}
	}
}

func createTenant(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.Args()[0]

	logrus.Infof("Creating tenant: %s", tenant)

	errCheck(ctx, getClient(ctx).TenantPost(&contivClient.Tenant{
		TenantName: tenant,
	}))
}

func deleteTenant(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.Args()[0]

	logrus.Infof("Deleting tenant %s", tenant)

	errCheck(ctx, getClient(ctx).TenantDelete(tenant))
}

func listTenants(ctx *cli.Context) {
	argCheck(0, ctx)

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

func createEndpointGroup(ctx *cli.Context) {
	argCheck(2, ctx)

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]
	group := ctx.Args()[1]

	policies := strings.Split(ctx.String("policy"), ",")
	if ctx.String("policy") == "" {
		policies = []string{}
	}

	errCheck(ctx, getClient(ctx).EndpointGroupPost(&contivClient.EndpointGroup{
		TenantName:  tenant,
		NetworkName: network,
		GroupName:   group,
		Policies:    policies,
	}))
}

func deleteEndpointGroup(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	group := ctx.Args()[0]

	errCheck(ctx, getClient(ctx).EndpointGroupDelete(tenant, group))
}

func listEndpointGroups(ctx *cli.Context) {
	argCheck(0, ctx)

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
		writer.Write([]byte("Tenant\tGroup\tNetwork\tPolicies\n"))
		writer.Write([]byte("------\t-----\t-------\t--------\n"))
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
				[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\n",
					group.TenantName,
					group.GroupName,
					group.NetworkName,
					policies,
				)))
		}
	}
}

//addBgp is a netctl interface routine to add
//bgp config
func addBgp(ctx *cli.Context) {
	argCheck(1, ctx)

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
	argCheck(1, ctx)

	hostname := ctx.Args()[0]
	logrus.Infof("Deleting Bgp router config %s:%s", hostname)

	errCheck(ctx, getClient(ctx).BgpDelete(hostname))
}

//listBgpNeighbors is netctl interface routine to list
//Bgp neighbor configs for a given host
func listBgp(ctx *cli.Context) {
	argCheck(1, ctx)

	hostname := ctx.Args()[0]

	bgpList, err := getClient(ctx).BgpList()
	errCheck(ctx, err)

	filtered := []*contivClient.Bgp{}

	for _, host := range *bgpList {
		if host.Hostname == hostname || ctx.Bool("all") {
			filtered = append(filtered, host)
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, filtered)
	} else {

		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("HostName\tRouterIP\tAS\tNeighbor\tNeighborAS\n"))
		writer.Write([]byte("---------\t--------\t-------\t--------\t-------\n"))
		for _, group := range filtered {
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

func showGlobal(ctx *cli.Context) {
	argCheck(0, ctx)

	list, err := getClient(ctx).GlobalList()
	errCheck(ctx, err)

	if ctx.Bool("json") {
		dumpJSONList(ctx, list)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		defer writer.Flush()
		for _, gl := range *list {
			writer.Write([]byte(fmt.Sprintf("Fabric mode: %v\n", gl.NetworkInfraType)))
			writer.Write([]byte(fmt.Sprintf("Vlan Range: %v\n", gl.Vlans)))
			writer.Write([]byte(fmt.Sprintf("Vxlan range: %v\n", gl.Vxlans)))
		}
	}
}

func setGlobal(ctx *cli.Context) {
	fabMode := ctx.String("fabric-mode")
	vlans := ctx.String("vlan-range")
	vxlans := ctx.String("vxlan-range")

	errCheck(ctx, getClient(ctx).GlobalPost(&contivClient.Global{
		Name:             "global",
		NetworkInfraType: fabMode,
		Vlans:            vlans,
		Vxlans:           vxlans,
	}))
}

func dumpJSONList(ctx *cli.Context, list interface{}) {
	content, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stdout.Write(content)
	os.Stdout.WriteString("\n")
}

func showVersion(ctx *cli.Context) {
	argCheck(0, ctx)

	ver := version.Info{}
	if err := getObject(ctx, versionURL(ctx), &ver); err != nil {
		fmt.Printf("Unable to fetch version information")
	} else {
		fmt.Printf("Client Version:\n")
		fmt.Printf(version.String())
		fmt.Printf("\n")
		fmt.Printf("Server Version:\n")
		fmt.Printf(version.StringFromInfo(&ver))
	}
}

func createAppProfile(ctx *cli.Context) {
	argCheck(1, ctx)

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

func updateAppProfile(ctx *cli.Context) {
	argCheck(1, ctx)

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
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	prof := ctx.Args()[0]

	errCheck(ctx, getClient(ctx).AppProfileDelete(tenant, prof))
}

func listAppProfiles(ctx *cli.Context) {
	argCheck(0, ctx)

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
		writer.Write([]byte("------\t-----\t---------\n"))
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
	argCheck(1, ctx)

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
	argCheck(1, ctx)

	serviceName := ctx.Args()[0]
	serviceSubnet := ctx.String("network")
	tenantName := ctx.String("tenant")

	selectors := ctx.StringSlice("selector")
	ports := ctx.StringSlice("port")
	ipAddress := ctx.String("preferred-ip")
	errCheck(ctx, getClient(ctx).ServiceLBPost(&contivClient.ServiceLB{
		ServiceName: serviceName,
		TenantName:  tenantName,
		NetworkName: serviceSubnet,
		Selectors:   selectors,
		Ports:       ports,
		IpAddress:   ipAddress,
	}))
}

//deleteServiceLB is a netctl interface routine to delete
//service object
func deleteServiceLB(ctx *cli.Context) {
	argCheck(1, ctx)

	serviceName := ctx.Args()[0]
	tenantName := ctx.String("tenant")

	fmt.Printf("Deleting Service  %s,%s", serviceName, tenantName)

	errCheck(ctx, getClient(ctx).ServiceLBDelete(serviceName, tenantName))
}

//listServiceLB is a netctl interface routine to delete
//service object
func listServiceLB(ctx *cli.Context) {
	argCheck(1, ctx)

	serviceName := ctx.Args()[0]
	tenantName := ctx.String("tenantName")

	svcList, err := getClient(ctx).ServiceLBList()
	errCheck(ctx, err)

	filtered := []*contivClient.ServiceLB{}

	for _, svc := range *svcList {
		if svc.ServiceName == serviceName || ctx.Bool("all") && svc.TenantName == tenantName {
			filtered = append(filtered, svc)
		}
	}

	if ctx.Bool("json") {
		dumpJSONList(ctx, filtered)
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
