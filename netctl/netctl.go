package netctl

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/contiv/netplugin/version"
)

// DefaultMaster is the master to use when none is provided.
const DefaultMaster = "http://localhost:9999"

func createPolicy(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	logrus.Infof("Creating policy %s:%s", tenant, policy)

	url := fmt.Sprintf("%s%s:%s/", policyURL(ctx), tenant, policy)

	postMap(ctx, url, map[string]interface{}{
		"tenantName": tenant,
		"policyName": policy,
	})
}

func deletePolicy(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	logrus.Infof("Deleting policy %s:%s", tenant, policy)

	url := fmt.Sprintf("%s%s:%s/", policyURL(ctx), tenant, policy)
	deleteURL(ctx, url)
}

func listPolicies(ctx *cli.Context) {
	argCheck(0, ctx)

	tenant := ctx.String("tenant")

	policies := getList(ctx, policyURL(ctx))
	filtered := []map[string]interface{}{}

	if !ctx.Bool("all") {
		for _, policy := range policies {
			if policy["tenantName"] == tenant {
				filtered = append(filtered, policy)
			}
		}

		if ctx.Bool("json") {
			dumpList(ctx, filtered)
		} else if ctx.Bool("quiet") {
			policies := ""
			for _, policy := range filtered {
				policies += policy["policyName"].(string) + "\n"
			}
			os.Stdout.WriteString(policies)
		} else {
			writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			defer writer.Flush()
			writer.Write([]byte("Tenant\tPolicy\n"))
			writer.Write([]byte("------\t------\n"))

			for _, policy := range filtered {
				writer.Write([]byte(fmt.Sprintf("%s\t%s\n", policy["tenantName"].(string), policy["policyName"].(string))))
			}
		}
	}
}

func addRule(ctx *cli.Context) {
	argCheck(2, ctx)

	args := map[string]interface{}{
		"tenantName":    ctx.String("tenant"),
		"policyName":    ctx.Args()[0],
		"ruleId":        ctx.Args()[1],
		"priority":      ctx.Int("priority"),
		"direction":     ctx.String("direction"),
		"endpointGroup": ctx.String("epg"),
		"network":       ctx.String("network"),
		"ipAddress":     ctx.String("ip-address"),
		"protocol":      ctx.String("protocol"),
		"port":          ctx.Int("port"),
		"action":        ctx.String("action"),
	}

	url := fmt.Sprintf(
		"%s%s:%s:%s/",
		ruleURL(ctx),
		args["tenantName"].(string),
		args["policyName"].(string),
		args["ruleId"].(string),
	)

	postMap(ctx, url, args)
}

func deleteRule(ctx *cli.Context) {
	argCheck(2, ctx)

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]
	ruleID := ctx.Args()[1]

	deleteURL(ctx, fmt.Sprintf("%s%s:%s:%s/", ruleURL(ctx), tenant, policy, ruleID))
}

func listRules(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	policy := ctx.Args()[0]

	rules := getList(ctx, ruleURL(ctx))

	writeRules := map[float64][]map[string]interface{}{}

	writePrio := []float64{}

	for _, rule := range rules {
		if ctx.Bool("all") || (rule["tenantName"] == tenant && rule["policyName"] == policy) {
			prio := rule["priority"].(float64)

			if _, ok := writeRules[prio]; !ok {
				writeRules[prio] = []map[string]interface{}{}
				writePrio = append(writePrio, prio)
			}

			writeRules[prio] = append(writeRules[prio], rule)
		}
	}

	sort.Float64s(writePrio)

	results := []map[string]interface{}{}

	for _, prio := range writePrio {
		for _, rule := range writeRules[prio] {
			results = append(results, rule)
		}
	}

	if ctx.Bool("json") {
		dumpList(ctx, results)
	} else if ctx.Bool("quiet") {
		rules := ""
		for _, rule := range results {
			rules += rule["ruleId"].(string) + "\n"
		}
		os.Stdout.WriteString(rules)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Rule\tDirection\tPriority\tEndpointGroup\tNetwork\tIpAddress\tProtocol\tPort\tAction\n"))
		writer.Write([]byte("----\t---------\t--------\t-------------\t-------\t---------\t--------\t----\t------\n"))

		for _, rule := range results {
			writer.Write([]byte(fmt.Sprintf(
				"%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
				rule["ruleId"],
				rule["direction"],
				rule["priority"],
				rule["endpointGroup"],
				rule["network"],
				rule["ipAddress"],
				rule["protocol"],
				rule["port"],
				rule["action"],
			)))
		}
	}
}

func createNetwork(ctx *cli.Context) {
	argCheck(1, ctx)

	subnet := ctx.String("subnet")
	gateway := ctx.String("gateway")

	if subnet == "" || gateway == "" {
		errExit(ctx, exitHelp, "Invalid Arguments", true)
	}

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]
	encap := ctx.String("encap")
	pktTag := ctx.Int("pkt-tag")

	url := fmt.Sprintf("%s%s:%s/", networkURL(ctx), tenant, network)

	out := map[string]interface{}{
		"tenantName":  tenant,
		"networkName": network,
		"encap":       encap,
		"pktTag":      pktTag,
		"subnet":      subnet,
		"gateway":     gateway,
	}

	postMap(ctx, url, out)
}

func deleteNetwork(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]

	logrus.Infof("Deleting network %s:%s", tenant, network)

	deleteURL(ctx, fmt.Sprintf("%s%s:%s/", networkURL(ctx), tenant, network))
}

func listNetworks(ctx *cli.Context) {
	argCheck(0, ctx)

	tenant := ctx.String("tenant")

	list := getList(ctx, networkURL(ctx))

	filtered := []map[string]interface{}{}

	if ctx.Bool("all") {
		filtered = list
	} else {
		for _, net := range list {
			if net["tenantName"] == tenant {
				filtered = append(filtered, net)
			}
		}
	}

	if ctx.Bool("json") {
		dumpList(ctx, filtered)
	} else if ctx.Bool("quiet") {
		networks := ""
		for _, network := range filtered {
			networks += network["networkName"].(string) + "\n"
		}
		os.Stdout.WriteString(networks)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Tenant\tNetwork\tEncap type\tPacket tag\tSubnet\tGateway\n"))
		writer.Write([]byte("------\t-------\t----------\t----------\t-------\t------\n"))

		for _, net := range filtered {
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\t%v\t%v\n",
					net["tenantName"],
					net["networkName"],
					net["encap"],
					net["pktTag"],
					net["subnet"],
					net["gateway"],
				)))
		}
	}
}

func createTenant(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.Args()[0]

	logrus.Infof("Creating tenant: %s", tenant)

	url := fmt.Sprintf("%s%s/", tenantURL(ctx), tenant)
	args := map[string]interface{}{
		"key":        tenant,
		"tenantName": tenant,
	}

	postMap(ctx, url, args)
}

func deleteTenant(ctx *cli.Context) {
	argCheck(1, ctx)

	tenant := ctx.Args()[0]

	logrus.Infof("Deleting tenant %s", tenant)

	url := fmt.Sprintf("%s%s/", tenantURL(ctx), tenant)
	deleteURL(ctx, url)
}

func listTenants(ctx *cli.Context) {
	argCheck(0, ctx)

	list := getList(ctx, tenantURL(ctx))

	if ctx.Bool("json") {
		dumpList(ctx, list)
	} else if ctx.Bool("quiet") {
		tenants := ""
		for _, tenant := range list {
			tenants += tenant["tenantName"].(string) + "\n"
		}
		os.Stdout.WriteString(tenants)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Name\t\n"))
		writer.Write([]byte("------\t\n"))

		for _, tenant := range list {
			writer.Write(
				[]byte(fmt.Sprintf("%v\t\n",
					tenant["tenantName"],
				)))
		}
	}
}

func createEndpointGroup(ctx *cli.Context) {
	argCheck(2, ctx)

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]
	group := ctx.Args()[1]
	url := fmt.Sprintf("%s%s:%s:%s/", epgURL(ctx), tenant, network, group)

	policies := strings.Split(ctx.String("policy"), ",")
	if ctx.String("policy") == "" {
		policies = []string{}
	}

	out := map[string]interface{}{
		"tenantName":  tenant,
		"groupName":   group,
		"networkName": network,
		"policies":    policies,
	}

	postMap(ctx, url, out)
}

func deleteEndpointGroup(ctx *cli.Context) {
	argCheck(2, ctx)

	tenant := ctx.String("tenant")
	network := ctx.Args()[0]
	group := ctx.Args()[1]

	deleteURL(ctx, fmt.Sprintf("%s%s:%s:%s/", epgURL(ctx), tenant, network, group))
}

func listEndpointGroups(ctx *cli.Context) {
	argCheck(0, ctx)

	tenant := ctx.String("tenant")

	list := getList(ctx, epgURL(ctx))

	filtered := []map[string]interface{}{}

	for _, group := range list {
		if group["tenantName"] == tenant || ctx.Bool("all") {
			filtered = append(filtered, group)
		}
	}

	if ctx.Bool("json") {
		dumpList(ctx, filtered)
	} else if ctx.Bool("quiet") {
		epgs := ""
		for _, epg := range filtered {
			epgs += epg["groupName"].(string) + "\n"
		}
		os.Stdout.WriteString(epgs)
	} else {

		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("Tenant\tGroup\tNetwork\tPolicies\n"))
		writer.Write([]byte("------\t-----\t-------\t--------\n"))
		for _, group := range filtered {
			policies := ""
			if group["policies"] != nil {
				policyList := []string{}
				for _, p := range group["policies"].([]interface{}) {
					policyList = append(policyList, p.(string))
				}
				policies = strings.Join(policyList, ",")
			}
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\t%v\n",
					group["tenantName"],
					group["groupName"],
					group["networkName"],
					policies,
				)))
		}
	}
}

func showGlobal(ctx *cli.Context) {
	argCheck(0, ctx)

	list := getList(ctx, globalURL(ctx))

	if ctx.Bool("json") {
		dumpList(ctx, list)
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		defer writer.Flush()
		for _, gl := range list {
			writer.Write([]byte(fmt.Sprintf("Fabric mode: %v\n", gl["network-infra-type"])))
			writer.Write([]byte(fmt.Sprintf("Vlan Range: %v\n", gl["vlans"])))
			writer.Write([]byte(fmt.Sprintf("Vxlan range: %v\n", gl["vxlans"])))
		}
	}
}

func setGlobal(ctx *cli.Context) {
	url := fmt.Sprintf("%sglobal/", globalURL(ctx))

	fabMode := ctx.String("fabric-mode")
	vlans := ctx.String("vlan-range")
	vxlans := ctx.String("vxlan-range")

	out := map[string]interface{}{
		"name":               "global",
		"network-infra-type": fabMode,
		"vlans":              vlans,
		"vxlans":             vxlans,
	}

	postMap(ctx, url, out)
}

func dumpList(ctx *cli.Context, list []map[string]interface{}) {
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

//addBgpNeighbors is a netctl interface routine to add
//bgp neighbor
func addBgpNeighbors(ctx *cli.Context) {
	argCheck(0, ctx)

	hostname := ctx.String("host")
	asid := ctx.String("as")

	neighbor := ctx.String("neighbor")
	url := fmt.Sprintf("%s%s/", bgpURL(ctx), hostname)

	out := map[string]interface{}{
		"Hostname": hostname,
		"as":       asid,
		"neighbor": neighbor,
	}
	postMap(ctx, url, out)
}

//deleteBgpNeighbors is a netctl interface routine to delete
//bgp neighbor
func deleteBgpNeighbors(ctx *cli.Context) {
	argCheck(0, ctx)

	hostname := ctx.String("host")
	logrus.Infof("Deleting router config %s:%s", hostname)

	deleteURL(ctx, fmt.Sprintf("%s%s/", bgpURL(ctx), hostname))
}

//listBgpNeighbors is netctl interface routine to list
//Bgp neighbor configs for a given host
func listBgpNeighbors(ctx *cli.Context) {
	argCheck(0, ctx)

	hostname := ctx.String("host")

	list := getList(ctx, bgpURL(ctx))
	filtered := []map[string]interface{}{}

	for _, group := range list {
		if group["hostname"] == hostname || ctx.Bool("all") {
			filtered = append(filtered, group)
		}
	}

	if ctx.Bool("json") {
		dumpList(ctx, filtered)
	} else {

		writer := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		defer writer.Flush()
		writer.Write([]byte("HostName\tNeighbor\tAS\n"))
		writer.Write([]byte("---------\t--------\t-------\n"))
		for _, group := range filtered {
			fmt.Println(group)
			writer.Write(
				[]byte(fmt.Sprintf("%v\t%v\t%v\t\n",
					group["host"],
					group["neighbor"],
					group["AS"],
				)))
		}
	}
}
