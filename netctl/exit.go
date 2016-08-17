package netctl

import (
	"fmt"
	"net/http"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

const (
	exitSuccess int = 0
	exitHelp        = iota
	exitRequest     = iota
	exitInvalid     = iota
	exitIO          = iota
)

func errExit(ctx *cli.Context, exitCode int, err string, showHelp bool) {
	if err != "" {
		logrus.Error(err)
	}

	if showHelp {
		cli.ShowSubcommandHelp(ctx)
	}

	os.Exit(exitCode)
}

func respCheck(resp *http.Response, ctx *cli.Context) {
	if resp.StatusCode != 200 {
		writeBody(resp, ctx)
		errExit(ctx, exitInvalid, fmt.Sprintf("Status %d in request response", resp.StatusCode), false)
	}
}

func errCheck(ctx *cli.Context, err error) {
	if err != nil {
		errExit(ctx, exitInvalid, err.Error(), false)
	}
}
