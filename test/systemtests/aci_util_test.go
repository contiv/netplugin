package systemtests

import (
	"bytes"
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// EPSpec for aci-gw
type EPSpec struct {
	Tenant string `json:"tenant,omitempty"`
	App    string `json:"app,omitempty"`
	Epg    string `json:"epg,omitempty"`
	EpMac  string `json:"epmac,omitempty"`
}

// EPResp from aci-gw
type EPResp struct {
	Result string `json:"result,omitempty"`
	Msg    string `json:"msg,omitempty"`
	IP     string `json:"ip,omitempty"`
	Vlan   string `json:"vlan,omitempty"`
}

func aciHTTPGet(url string, jin, jout interface{}) error {

	buf, err := json.Marshal(jin)
	if err != nil {
		return err
	}

	body := bytes.NewBuffer(buf)
	r, err := http.Post(url, "application/json", body)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	switch {
	case r.StatusCode == int(404):
		return errors.New("page not found")
	case r.StatusCode == int(403):
		return errors.New("access denied")
	case r.StatusCode == int(500):
		response, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return err
		}

		return errors.New(string(response))

	case r.StatusCode != int(200):
		log.Errorf("GET Status '%s' status code %d \n", r.Status, r.StatusCode)
		return errors.New(r.Status)
	}

	response, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(response, jout)
}

// GetEPFromAPIC checks learning
func GetEPFromAPIC(tenant, app, epg, mac string) (string, string, error) {
	ep := &EPSpec{Tenant: tenant,
		App:   app,
		Epg:   epg,
		EpMac: mac,
	}

	resp := &EPResp{}

	err := aciHTTPGet("http://localhost:5000/getEndpoint", ep, resp)

	if err == nil {
		if resp.Result == "success" {
			log.Infof("resp: %+v", resp)
			return resp.IP, resp.Vlan, nil
		}

		return resp.IP, resp.Vlan, errors.New(resp.Result)
	}

	log.Errorf("err: %v", err)
	return "", "", err

}

func (s *systemtestSuite) checkACILearning(tenant, app, epg string, containers []*container) error {

	ip := ""
	for _, c := range containers {
		mac, err := c.node.exec.getMACAddr(c, "eth0")
		mac = strings.ToUpper(mac)
		if err != nil {
			return err
		}

		containerIP, err := c.node.exec.getIPAddr(c, "eth0")
		if err != nil {
			return err
		}

		log.Infof("Checking IP %s and MAC %s learned on ACI...", containerIP, mac)

		for ix := 0; ix < 20; ix++ {
			ip, _, err = GetEPFromAPIC(tenant, app, epg, mac)
			if err == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if err != nil {
			return err
		}

		if ip != containerIP {
			log.Errorf("ip from apic: %s from container: %s", ip, containerIP)
			return errors.New("ip mismatch")
		}
	}
	return nil
}
