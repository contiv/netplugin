package netctl

import (
	"bytes"
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

func policyURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/policys/", baseURL(ctx))
}

func epgURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/endpointGroups/", baseURL(ctx))
}

func networkURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/networks/", baseURL(ctx))
}

func tenantURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/tenants/", baseURL(ctx))
}

func ruleURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/rules/", baseURL(ctx))
}

func versionURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/version", baseURL(ctx))
}

func globalURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/globals/", baseURL(ctx))
}

func bgpURL(ctx *cli.Context) string {
	return fmt.Sprintf("%s/api/Bgps/", baseURL(ctx))
}

func writeBody(resp *http.Response, ctx *cli.Context) {
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
	os.Stderr.Write(content)
}

func postMap(ctx *cli.Context, url string, jsonMap map[string]interface{}) {
	content, err := json.Marshal(jsonMap)
	handleBasicError(ctx, err)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(content))
	handleBasicError(ctx, err)

	respCheck(resp, ctx)
}

func deleteURL(ctx *cli.Context, url string) {
	req, err := http.NewRequest("DELETE", url, nil)
	handleBasicError(ctx, err)

	resp, err := client.Do(req)
	handleBasicError(ctx, err)

	respCheck(resp, ctx)
}

func getList(ctx *cli.Context, url string) []map[string]interface{} {
	resp, err := client.Get(url)
	handleBasicError(ctx, err)

	respCheck(resp, ctx)

	content, err := ioutil.ReadAll(resp.Body)
	handleBasicError(ctx, err)

	list := []map[string]interface{}{}

	handleBasicError(ctx, json.Unmarshal(content, &list))

	return list
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
