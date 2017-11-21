package netctl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"github.com/codegangsta/cli"
	contivClient "github.com/contiv/netplugin/contivmodel/client"
)

// Config represents the contents of ~/.netctl/config.json
type Config struct {
	Token string `json:"token"`
}

// applyConfig applies the netctl config to the specified ContivClient
func applyConfig(cl *contivClient.ContivClient) error {
	data, err := ioutil.ReadFile(configPath())
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	nc := Config{}
	if err := json.Unmarshal(data, &nc); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// add the token header we use to authenticate
	if err := cl.SetAuthToken(nc.Token); err != nil {
		return fmt.Errorf("failed to set auth token: %v", err)
	}

	return nil
}

func configExists(ctx *cli.Context) bool {
	if _, err := os.Stat(configPath()); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		errExit(ctx, exitIO, err.Error(), false)
		return false // compiler does not realize flow can't reach here
	}
}

// configPath returns the full path to the user's netctl config file
func configPath() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	return usr.HomeDir + "/.netctl/config.json"
}

// writeConfig writes out the netctl config file, creating the directory and file as necessary
func writeConfig(ctx *cli.Context, data []byte) {
	dir := filepath.Dir(configPath())

	// try to create the netctl config directory but ignore "already exists" errors.
	// only the user should be able to read the contents of this directory
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		errExit(ctx, exitIO, err.Error(), false)
	}

	// only the user should be able to read the config file
	if err := ioutil.WriteFile(configPath(), data, 0600); err != nil {
		errExit(ctx, exitIO, err.Error(), false)
	}
}
