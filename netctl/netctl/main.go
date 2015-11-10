package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/netplugin/netctl"
)

func main() {
	app := cli.NewApp()
	app.Flags = netctl.NetmasterFlags
	app.Version = ""
	app.Commands = netctl.Commands
	app.Run(os.Args)
}
