package main

import (
	"os"

	"github.com/codegangsta/cli"
	"github.com/contiv/netplugin/netctl"
	"github.com/contiv/netplugin/version"
)

func main() {
	app := cli.NewApp()
	app.Flags = netctl.NetmasterFlags
	app.Version = "\n" + version.String()
	app.Commands = netctl.Commands
	app.Run(os.Args)
}
