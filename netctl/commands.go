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
		Name:  "group",
		Usage: "Endpoint Group manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "create",
				Usage:     "Create an endpoint group",
				ArgsUsage: "[network] [group]",
				Flags: []cli.Flag{
					tenantFlag,
					cli.StringFlag{
						Name:  "policy, p",
						Usage: "Policy List (separated by commas)",
					},
				},
				Action: createEndpointGroup,
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
					cli.StringFlag{
						Name:  "pkt-tag, p",
						Usage: "Packet tag (Vlan/Vxlan ids)",
					},
					cli.StringFlag{
						Name:  "subnet, s",
						Usage: "Subnet CIDR - REQUIRED",
					},
					cli.StringFlag{
						Name:  "gateway, g",
						Usage: "Gateway",
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
				Flags:     []cli.Flag{quietFlag},
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
				Name:      "rule-ls",
				Usage:     "List rules for a given tenant,policy",
				ArgsUsage: "[policy]",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
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
				Name:      "set",
				Usage:     "Set global parameters",
				ArgsUsage: " ",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "fabric-mode, f",
						Usage: "Fabric mode (aci, aci-opflex or default)",
						Value: "default",
					},
					cli.StringFlag{
						Name:  "vlan-range, v",
						Usage: "Allowed Vlan id range",
						Value: "1-4094",
					},
					cli.StringFlag{
						Name:  "vxlan-range, x",
						Usage: "Allowed Vxlan VNID range",
						Value: "1-10000",
					},
				},
				Action: setGlobal,
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
						Name:  "hostname",
						Usage: "host name",
					},
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
		Usage: "service object creation",
		Subcommands: []cli.Command{
			{
				Name:      "ls",
				Aliases:   []string{"list"},
				Usage:     "List service objects",
				ArgsUsage: "[servicename]",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag, quietFlag},
				Action:    listServiceLB,
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
				Usage:     "Create Service object.",
				ArgsUsage: "[servicename]",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "tenant,t",
						Usage: "service tenant",
					},
					cli.StringFlag{
						Name:  "network,s",
						Usage: "service subnet",
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
