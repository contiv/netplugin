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
				Name:      "delete",
				Usage:     "Delete an endpoint group",
				ArgsUsage: "[network] [group]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deleteEndpointGroup,
			},
			{
				Name:      "list",
				Usage:     "List endpoint groups",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag},
				Action:    listEndpointGroups,
			},
		},
	},
	{
		Name:  "net",
		Usage: "Network manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "list",
				Usage:     "List networks",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag},
				Action:    listNetworks,
			},
			{
				Name:      "delete",
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
						Name:  "encap, e",
						Usage: "Encap type (vlan or vxlan)",
						Value: "vxlan",
					},
					cli.StringFlag{
						Name:  "pkt-tag, p",
						Usage: "Packet tag (Vlan/Vxlan ids)- REQUIRED",
					},
					cli.StringFlag{
						Name:  "subnet, s",
						Usage: "Subnet CIDR - REQUIRED",
					},
					cli.StringFlag{
						Name:  "gateway, g",
						Usage: "Gateway - REQUIRED",
					},
				},
				Action: createNetwork,
			},
		},
	},
	{
		Name:  "rule",
		Usage: "Rule manipulation tools",
		Subcommands: []cli.Command{
			{
				Name:      "list",
				Usage:     "List rules for a given tenant/policy",
				ArgsUsage: "[policy]",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag},
				Action:    listRules,
			},
			{
				Name:      "delete",
				Usage:     "Delete an existing rule.",
				ArgsUsage: "[policy] [rule id]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deleteRule,
			},
			{
				Name:      "add",
				Usage:     "Add a new rule.",
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
						Value: "both",
					},
					cli.StringFlag{
						Name:  "group, g",
						Usage: "Endpoint Group Name",
					},
					cli.StringFlag{
						Name:  "network, n",
						Usage: "Network name",
					},
					cli.StringFlag{
						Name:  "ip-address, a",
						Usage: "IP Address",
					},
					cli.StringFlag{
						Name:  "protocol, l",
						Usage: "Protocol (e.g., tcp)",
					},
					cli.IntFlag{
						Name:  "port, P",
						Usage: "Port",
					},
					cli.StringFlag{
						Name:  "action, j",
						Usage: "Action to take (e.g., deny)",
						Value: "accept",
					},
				},
				Action: addRule,
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
				Name:      "delete",
				Usage:     "Delete a policy",
				ArgsUsage: "[policy]",
				Flags:     []cli.Flag{tenantFlag},
				Action:    deletePolicy,
			},
			{
				Name:      "list",
				Usage:     "List policies",
				ArgsUsage: " ",
				Flags:     []cli.Flag{tenantFlag, allFlag, jsonFlag},
				Action:    listPolicies,
			},
		},
	},
}
