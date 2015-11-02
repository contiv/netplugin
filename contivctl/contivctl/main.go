package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/netplugin/contivctl"
)

func main() {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "master",
			Value:  contivctl.DefaultMaster,
			Usage:  "The hostname of the netmaster",
			EnvVar: "NETMASTER",
		},
	}

	app.Version = ""
	app.Commands = contivctl.Commands
	app.Run(os.Args)
}
