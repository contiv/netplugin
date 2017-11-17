package netctl

import "github.com/codegangsta/cli"

var tenantFlag = cli.StringFlag{
	Name:  "tenant, t",
	Value: "default",
	Usage: "Name of the tenant",
}

var allFlag = cli.BoolFlag{
	Name:  "all, a",
	Usage: "List all items",
}

var jsonFlag = cli.BoolFlag{
	Name:  "json, j",
	Usage: "Output list in JSON format",
}

var quietFlag = cli.BoolFlag{
	Name:  "quiet, q",
	Usage: "Only display name field",
}

// NetmasterFlags encapsulates the flags required for talking to the netmaster.
var NetmasterFlags = []cli.Flag{
	cli.StringFlag{
		Name:   "netmaster",
		Value:  DefaultMaster,
		Usage:  "The hostname of the netmaster",
		EnvVar: "NETMASTER",
	},
	cli.BoolFlag{
		Name:   "insecure",
		Usage:  "if true, strict certificate checking will be disabled",
		EnvVar: "INSECURE",
	},
}

// Commands are all the commands that go into `contivctl`, the end-user tool.
// These are represented as cli.Command objects.
var Commands = []cli.Command{
	{
		Name:   "version",
		Usage:  "Version Information",
		Action: showVersion,
	},
	{
		Name:   "login",
		Usage:  "authenticate to Contiv (you must specify auth_proxy's HTTPS address in the --netmaster flag)",
		Action: login,
	},
	{
		Name:  "group",
		Usage: "Endpoint Group manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "create",
				Usage:     "Create an endpoint group",
				ArgsUsage: "[network] [group]",
				Flags: []cli.Flag{
					tenantFlag,
					cli.StringSliceFlag{
						Name:  "policy, p",
						Usage: "Policy",
					},
					cli.StringFlag{
						Name:  "networkprofile, n",
						Usage: "network profile",
					},
					cli.StringSliceFlag{
						Name:  "external-contract, e",
						Usage: "External contract",
					},
					cli.StringFlag{
						Name:  "ip-pool, r",
						Usage: "IP Address range, example 10.36.0.1-10.36.0.10",
					},
					cli.StringFlag{
						Name:  "epg-tag, tag",
						Usage: "Configured Group Tag",
					},
				},
				Action: createEndpointGroup,
			},
			{
				Name:      "inspect",
				Usage:     "Inspect a EndpointGroup",
				ArgsUsage: "[group]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    inspectEndpointGroup,
			},
			{
				Name:      "rm",
				Aliases:   []string{"delete"},
				Usage:     "Delete an endpoint group",
				ArgsUsage: "[group]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deleteEndpointGroup,
			},
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List endpoint groups",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
				Action:    listEndpointGroups,
			},
		},
	},
	{
		Name:    "endpoint",
		Aliases: []string{"ep"},
		Usage:   "Endpoint Inspection",
		Subcommands: []cli.Command{
			{
				Name:      "inspect",
				Usage:     "Inspect an Endpoint",
				ArgsUsage: "[epid]",
				Action:    inspectEndpoint,
			},
		},
	},
	{
		Name:  "netprofile",
		Usage: "Network profile manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "create",
				Usage:     "Create a network profile",
				ArgsUsage: "[netprofile]",
				Flags: []cli.Flag{
					tenantFlag,
					cli.StringFlag{
						Name:  "bandwidth, b",
						Usage: "Bandwidth (e.g., 10 kbps, 100 mbps, 1gbps)",
					},
					cli.IntFlag{
						Name:  "dscp, d",
						Usage: "DSCP",
					},
					cli.IntFlag{
						Name:  "burst, s",
						Usage: "burst size(Must be in kilobytes)",
					},
				},
				Action: createNetProfile,
			},
			{
				Name:      "rm",
				Aliases:   []string{"delete"},
				Usage:     "Delete a network profile",
				ArgsUsage: "[network] [group]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deleteNetProfile,
			},
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List network profile",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
				Action:    listNetProfiles,
			},
			{
				Name:      "inspect",
				Usage:     "inspect network profile",
				ArgsUsage: "[netprofile]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    inspectNetprofile,
			},
		},
	},
	{
		Name:    "network",
		Aliases: []string{"net"},
		Usage:   "Network manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List networks",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
				Action:    listNetworks,
			},
			{
				Name:      "inspect",
				Usage:     "Inspect a Network",
				ArgsUsage: "[network]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    inspectNetwork,
			},
			{
				Name:      "rm",
				Aliases:   []string{"delete"},
				Usage:     "Delete a network",
				ArgsUsage: "[network]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deleteNetwork,
			},
			{
				Name:      "create",
				Usage:     "Create a network",
				ArgsUsage: "[network]",
				Flags: []cli.Flag{
					tenantFlag,
					cli.StringFlag{
						Name:  "nw-type, n",
						Usage: "Network Type (infra or data)",
						Value: "data",
					},
					cli.StringFlag{
						Name:  "encap, e",
						Usage: "Encap type (vlan or vxlan)",
						Value: "vxlan",
					},
					cli.IntFlag{
						Name:  "pkt-tag, p",
						Usage: "Packet tag (Vlan ID/VNI)",
					},
					cli.StringFlag{
						Name:  "subnet, s",
						Usage: "Subnet CIDR - REQUIRED",
					},
					cli.StringFlag{
						Name:  "gateway, g",
						Usage: "Gateway",
					},
					cli.StringFlag{
						Name:  "subnetv6, s6",
						Usage: "IPv6 Subnet CIDR ",
					},
					cli.StringFlag{
						Name:  "gatewayv6, g6",
						Usage: "IPv6 Gateway",
					},
					cli.StringFlag{
						Name:  "nw-tag, tag",
						Usage: "Configured Network Tag",
					},
				},
				Action: createNetwork,
			},
		},
	},
	{
		Name:  "tenant",
		Usage: "Tenant manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List tenants",
				ArgsUsage: " ",
				Flags:     []cli.Flag{quietFlag, jsonFlag},
				Action:    listTenants,
			},
			{
				Name:      "rm",
				Aliases:   []string{"delete"},
				Usage:     "Delete a tenant",
				ArgsUsage: "[tenant]",
				Action:    deleteTenant,
			},
			{
				Name:      "create",
				Usage:     "Create a tenant",
				ArgsUsage: "[tenant]",
				Action:    createTenant,
			},
			{
				Name:      "inspect",
				Usage:     "Inspect a tenant",
				ArgsUsage: "[tenant]",
				Action:    inspectTenant,
			},
		},
	},
	{
		Name:  "policy",
		Usage: "Policy manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "create",
				Usage:     "Create a new policy",
				ArgsUsage: "[policy]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    createPolicy,
			},
			{
				Name:      "rm",
				Aliases:   []string{"delete"},
				Usage:     "Delete a policy",
				ArgsUsage: "[policy]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deletePolicy,
			},
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List policies",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
				Action:    listPolicies,
			},
			{
				Name:      "inspect",
				Usage:     "Inspect a policy",
				ArgsUsage: "[policy]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    inspectPolicy,
			},
			{
				Name:      "rule-ls",
				Usage:     "List rules for a given tenant,policy",
				ArgsUsage: "[policy]",
				Flags:     []cli.Flag{tenantFlag, jsonFlag, quietFlag},
				Action:    listRules,
			},
			{
				Name:      "rule-rm",
				Usage:     "Delete a rule from the policy",
				ArgsUsage: "[policy] [rule id]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deleteRule,
			},
			{
				Name:      "rule-add",
				Usage:     "Add a new rule to the policy",
				ArgsUsage: "[policy] [rule id]",
				Flags: []cli.Flag{
					tenantFlag,
					cli.IntFlag{
						Name:  "priority, p",
						Usage: "Priority Indicator",
						Value: 1,
					},
					cli.StringFlag{
						Name:  "direction, d",
						Usage: "Direction of traffic (in/out)",
						Value: "in",
					},
					cli.StringFlag{
						Name:  "from-group, g",
						Usage: "From Endpoint Group Name (Valid in incoming direction only)",
					},
					cli.StringFlag{
						Name:  "to-group, e",
						Usage: "To Endpoint Group Name (Valid in outgoing direction only)",
					},
					cli.StringFlag{
						Name:  "from-network, n",
						Usage: "From Network name (Valid in incoming direction only)",
					},
					cli.StringFlag{
						Name:  "to-network, o",
						Usage: "To Network name (Valid in outgoing direction only)",
					},
					cli.StringFlag{
						Name:  "from-ip-address, i",
						Usage: "From IP address/CIDR (Valid in incoming direction only)",
					},
					cli.StringFlag{
						Name:  "to-ip-address, s",
						Usage: "To IP address/CIDR (Valid in outgoing direction only)",
					},
					cli.StringFlag{
						Name:  "protocol, l",
						Usage: "Protocol (e.g., tcp, udp, icmp)",
					},
					cli.IntFlag{
						Name:  "port, P",
						Usage: "Port",
					},
					cli.StringFlag{
						Name:  "action, j",
						Usage: "Action to take (allow or deny)",
						Value: "allow",
					},
				},
				Action: addRule,
			},
		},
	},
	{
		Name:  "external-contracts",
		Usage: "External contracts",
		Subcommands: []cli.Command{
			{
				Name:    "ls",
				Aliases: []string{"list"},
				Usage:   "List external contracts",
				Flags:   []cli.Flag{quietFlag, allFlag, jsonFlag, tenantFlag},
				Action:  listExternalContracts,
			},
			{
				Name:    "rm",
				Aliases: []string{"delete"},
				Usage:   "Delete external contracts",
				Flags:   []cli.Flag{tenantFlag},
				Action:  deleteExternalContracts,
			},
			{
				Name:  "create",
				Usage: "Create external contracts",
				Flags: []cli.Flag{
					tenantFlag,
					cli.BoolFlag{
						Name:  "consumed, c",
						Usage: "External contracts type - consumed",
					},
					cli.BoolFlag{
						Name:  "provided, p",
						Usage: "External contracts type - provided",
					},
					cli.StringSliceFlag{
						Name:  "contract, a",
						Usage: "Contract",
					},
				},
				Action: createExternalContracts,
			},
		},
	},
	{
		Name:  "global",
		Usage: "Global information",
		Subcommands: []cli.Command{
			{
				Name:      "info",
				Usage:     "Show global information",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag},
				Action:    showGlobal,
			},
			{
				Name:      "inspect",
				Usage:     "Inspect Global Operational Information",
				ArgsUsage: " ",
				Action:    inspectGlobal,
			},
			{
				Name:      "set",
				Usage:     "Set global parameters",
				ArgsUsage: " ",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "fabric-mode, f",
						Usage: "Fabric mode (aci, aci-opflex or default)",
					},
					cli.StringFlag{
						Name:  "vlan-range, v",
						Usage: "Allowed Vlan id range",
					},
					cli.StringFlag{
						Name:  "vxlan-range, x",
						Usage: "Allowed Vxlan VNID range",
					},
					cli.StringFlag{
						Name:  "fwd-mode, b",
						Usage: "forwarding mode (bridge,routing)",
					},
					cli.StringFlag{
						Name:  "arp-mode, a",
						Usage: "arp mode (proxy,flood)",
					},
					cli.StringFlag{
						Name:  "private-subnet, s",
						Usage: "Select a /16 private subnet for host access",
						Value: "172.19.0.0/16",
					},
				},
				Action: setGlobal,
			},
		},
	},
	{
		Name:  "aci-gw",
		Usage: "ACI Gateway information",
		Subcommands: []cli.Command{
			{
				Name:      "info",
				Usage:     "Show aci gateway information",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag},
				Action:    showAciGw,
			},
			{
				Name:      "inspect",
				Usage:     "Inspect aci gateway operational information",
				ArgsUsage: " ",
				Action:    inspectAciGw,
			},
			{
				Name:      "set",
				Usage:     "Set aci-gw parameters",
				ArgsUsage: " ",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "path-bindings, p",
						Usage: "Blank or  Comma separated paths of the form topology/pod-1/paths-101/pathep-[eth1/14]",
					},
					cli.StringFlag{
						Name:  "node-bindings, n",
						Usage: "Blank or  Comma separated nodes of the form topology/pod-1/node-101",
					},
					cli.StringFlag{
						Name:  "phys-dom, d",
						Usage: "ACI physical domain name (e.g. containerDom)",
					},
					cli.StringFlag{
						Name:  "enforce-policies, e",
						Usage: "Should security policies be enforced (yes,no)",
						Value: "yes",
					},
					cli.StringFlag{
						Name:  "include-common-tenant, i",
						Usage: "Should gateway look up objects in common tenant as well(yes,no)",
						Value: "no",
					},
				},
				Action: setAciGw,
			},
		},
	},
	{
		Name:  "bgp",
		Usage: "router capability configuration",
		Subcommands: []cli.Command{
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List BGP configuration",
				ArgsUsage: "[hostname]",
				Flags:     []cli.Flag{jsonFlag, quietFlag},
				Action:    listBgp,
			},
			{
				Name:      "rm",
				Aliases:   []string{"delete"},
				Usage:     "Delete BGP configuration",
				ArgsUsage: "[hostname]",
				Flags:     []cli.Flag{},
				Action:    deleteBgp,
			},
			{
				Name:      "create",
				Usage:     "Add BGP configuration.",
				ArgsUsage: "[hostname]",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "router-ip",
						Usage: "BGP my-router ip ",
					},
					cli.StringFlag{
						Name:  "as",
						Usage: "Self AS id",
					},
					cli.StringFlag{
						Name:  "neighbor-as",
						Usage: "BGP neighbor AS id",
					},
					cli.StringFlag{
						Name:  "neighbor",
						Usage: "BGP neighbor to be added",
					},
				},
				Action: addBgp,
			},
			{
				Name:      "inspect",
				Usage:     "Inspect Bgp",
				ArgsUsage: "[hostname]",
				Action:    inspectBgp,
			},
		},
	},
	{
		Name:  "app-profile",
		Usage: "Application Profile manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "create",
				Usage:     "Create an application profile",
				ArgsUsage: "[app-profile]",
				Flags: []cli.Flag{
					tenantFlag,
					cli.StringFlag{
						Name:  "group, g",
						Usage: "Endpoint Group List (separated by commas)",
					},
				},
				Action: createAppProfile,
			},
			{
				Name:      "update",
				Usage:     "Update an application profile",
				ArgsUsage: "[app-profile]",
				Flags: []cli.Flag{
					tenantFlag,
					cli.StringFlag{
						Name:  "group, g",
						Usage: "Endpoint Group List (separated by commas)",
					},
				},
				Action: updateAppProfile,
			},
			{
				Name:      "rm",
				Aliases:   []string{"delete"},
				Usage:     "Delete an application profile",
				ArgsUsage: "[app-profile]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deleteAppProfile,
			},
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List Application Profiles",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
				Action:    listAppProfiles,
			},
			{
				Name:      "group-ls",
				Aliases:   []string{"group-list"},
				Usage:     "List groups in an app-profile",
				ArgsUsage: "[network] [app-profile]",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
				Action:    listAppProfEpgs,
			},
		},
	},
	{
		Name:  "service",
		Usage: "service object creation (only for  docker version <= 1.12.x)",
		Subcommands: []cli.Command{
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List service objects",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
				Action:    listServiceLB,
			},
			{
				Name:      "inspect",
				Usage:     "Inspect a Network",
				ArgsUsage: "[servicename]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    inspectServiceLb,
			},
			{
				Name:      "rm",
				Aliases:   []string{"delete"},
				Usage:     "Delete service object",
				ArgsUsage: "[servicename]",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "tenant,t",
						Usage: "service tenant",
					},
				},
				Action: deleteServiceLB,
			},
			{
				Name:      "create",
				Usage:     "Create Service object (only for docker version <= 1.12.x)",
				ArgsUsage: "[servicename]",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "tenant,t",
						Usage: "service tenant",
					},
					cli.StringFlag{
						Name:  "network,s",
						Usage: "service network name",
					},
					cli.StringSliceFlag{
						Name:  "selector,l",
						Usage: "service selector .Usage: --selector=key1=value1 --selector=key2=value2",
					},
					cli.StringSliceFlag{
						Name:  "port,p",
						Usage: "service/provider Port Usage- --port=svcPort1:provPort1:protocol --port=svcPort2:provPort2:protocol",
					},
					cli.StringFlag{
						Name:  "preferred-ip,ip",
						Usage: "preferred ip address",
					},
				},
				Action: createServiceLB,
			},
		},
	},
}
