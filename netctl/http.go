package netctl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/codegangsta/cli"
)

var client = &http.Client{}

func baseURL(ctx *cli.Context) string {
	return ctx.GlobalString("master")
}

func policyURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/policys/", baseURL(ctx))
}

func epgURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/endpointGroups/", baseURL(ctx))
}

func networkURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/networks/", baseURL(ctx))
}

func ruleURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/rules/", baseURL(ctx))
}

func postMap(ctx *cli.Context, url string, jsonMap map[string]interface{}) {
	content, err := json.Marshal(jsonMap)
	if err != nil {
		errExit(ctx, exitRequest, err.Error(), false)
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(content))
	if err != nil {
		errExit(ctx, exitRequest, err.Error(), false)
	}

	respCheck(resp, ctx)
}

func deleteURL(ctx *cli.Context, url string) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		errExit(ctx, exitRequest, err.Error(), false)
	}

	resp, err := client.Do(req)
	if err != nil {
		errExit(ctx, exitRequest, err.Error(), false)
	}

	respCheck(resp, ctx)
}

func getList(ctx *cli.Context, url string) []map[string]interface{} {
	resp, err := client.Get(url)
	if err != nil {
		errExit(ctx, exitRequest, err.Error(), false)
	}

	respCheck(resp, ctx)

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errExit(ctx, exitRequest, err.Error(), false)
	}

	list := []map[string]interface{}{}

	if err := json.Unmarshal(content, &list); err != nil {
		errExit(ctx, exitRequest, err.Error(), false)
	}

	return list
}
