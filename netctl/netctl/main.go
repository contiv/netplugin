package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/netplugin/netctl"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "master",
			Value:  netctl.DefaultMaster,
			Usage:  "The hostname of the netmaster",
			EnvVar: "NETMASTER",
		},
	}

	app.Version = ""
	app.Commands = netctl.Commands
	app.Run(os.Args)
}
