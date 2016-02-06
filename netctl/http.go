package netctl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/codegangsta/cli"
)

var client = &http.Client{}

func handleBasicError(ctx *cli.Context, err error) {
	if err != nil {
		errExit(ctx, exitRequest, err.Error(), false)
	}
}

func baseURL(ctx *cli.Context) string {
	return ctx.GlobalString("netmaster")
}

func versionURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/version", baseURL(ctx))
}

func writeBody(resp *http.Response, ctx *cli.Context) {
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stderr.Write(content)
}

func getObject(ctx *cli.Context, url string, jdata interface{}) error {
	resp, err := client.Get(url)
	handleBasicError(ctx, err)

	respCheck(resp, ctx)

	content, err := ioutil.ReadAll(resp.Body)
	handleBasicError(ctx, err)

	handleBasicError(ctx, json.Unmarshal(content, jdata))

	return nil
}
