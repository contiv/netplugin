package svcplugin

import (
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/contiv/netplugin/svcplugin/bridge"
)

// SvcregPlugin Service plugin interface
type SvcregPlugin interface {
	AddService(srvID string, srvName string, nwName string, tenantName string, srvIP string)
	RemoveService(srvID string, srvName string, nwName string, tenantName string, srvIP string)
}

// NewSvcregPlugin creates a new bridge to Service plugin
func NewSvcregPlugin(adapterURI string, bridgeCfg *bridge.Config) (SvcregPlugin, chan struct{}, error) {
	// quitCh is the channel to stop svc plugin
	var quitCh = make(chan struct{})

	log.Printf("Initing service plugin")
	var bConfig bridge.Config
	defaultBridgeConfig := bridge.DefaultBridgeConfig()

	// Check if we got a bridge config
	if bridgeCfg == nil {
		bConfig = defaultBridgeConfig
		bConfig.RefreshTTL = 15
		bConfig.RefreshInterval = 10
	} else {
		bConfig = *bridgeCfg
		if bConfig.HostIP != "" {
			log.Println("Forcing host IP to", bConfig.HostIP)
		}

		if bConfig.RefreshTTL < 0 {
			log.WithFields(log.Fields{
				"RefreshTTL": bConfig.RefreshTTL,
			}).Warn("RefreshTTL must be greater than 0.",
				"Setting to default value of ",
				defaultBridgeConfig.RefreshTTL,
				" and continuing")
			bConfig.RefreshTTL = 0
		}
		if bConfig.RefreshInterval < 0 {
			log.WithFields(log.Fields{
				"RefreshInterval": bConfig.RefreshInterval,
			}).Warn("RefreshInterval must be greater than 0.",
				"Setting to default value of ",
				defaultBridgeConfig.RefreshInterval,
				" and continuing")
			bConfig.RefreshInterval = 0
		}
		if (bConfig.RefreshTTL == 0 && bConfig.RefreshInterval > 0) ||
			(bConfig.RefreshTTL > 0 && bConfig.RefreshInterval == 0) {
			log.WithFields(log.Fields{
				"RefreshTTL":      bConfig.RefreshTTL,
				"RefreshInterval": bConfig.RefreshInterval,
			}).Warn("RefreshTTL and RefreshInterval ",
				"must be specified together or not at all. ",
				"Setting to default values (",
				defaultBridgeConfig.RefreshTTL,
				",", defaultBridgeConfig.RefreshInterval,
				") and continuing")
			bConfig.RefreshTTL = defaultBridgeConfig.RefreshTTL
			bConfig.RefreshInterval = defaultBridgeConfig.RefreshInterval
		} else if bConfig.RefreshTTL > 0 &&
			bConfig.RefreshTTL <= bConfig.RefreshInterval {
			log.WithFields(log.Fields{
				"RefreshTTL":      bConfig.RefreshTTL,
				"RefreshInterval": bConfig.RefreshInterval,
			}).Warn("RefreshTTL must be greater than RefreshInterval",
				". Setting to default values (",
				defaultBridgeConfig.RefreshTTL,
				",", defaultBridgeConfig.RefreshInterval,
				") and continuing")
			bConfig.RefreshTTL = defaultBridgeConfig.RefreshTTL
			bConfig.RefreshInterval = defaultBridgeConfig.RefreshInterval
		}

		if bConfig.DeregisterCheck != "always" &&
			bConfig.DeregisterCheck != "on-success" {
			log.WithFields(log.Fields{
				"DegisterCheck": bConfig.DeregisterCheck,
			}).Warn("Deregister must be \"always\" or ",
				"\"on-success\". Setting to default value of ",
				defaultBridgeConfig.DeregisterCheck,
				" and continuing")
			bConfig.DeregisterCheck = defaultBridgeConfig.DeregisterCheck
		}
	}

	log.Info("Creating a new bridge: ", adapterURI)

	b, err := bridge.New(adapterURI, bConfig)

	if err != nil {
		log.WithFields(log.Fields{
			"Error": err,
		}).Error("Bridge creation errored out. ",
			"Service registration will not work.")
		return nil, nil, err
	}

	// TODO: Add proper checks for retryInterval, retryAttempts
	attempt := 0
	for bConfig.RetryAttempts == -1 || attempt <= bConfig.RetryAttempts {
		log.Infof("Connecting to backend (%v/%v)",
			attempt, bConfig.RetryAttempts)

		err = b.Ping()
		if err == nil {
			break
		}

		if err != nil && attempt == bConfig.RetryAttempts {
			log.Errorf("Service plugin not connecting to backend %v", err)
		}

		time.Sleep(time.Duration(bConfig.RetryInterval) * time.Millisecond)
		attempt++
	}

	// Start the TTL refresh timer
	if bConfig.RefreshInterval > 0 {
		ticker := time.NewTicker(time.Duration(bConfig.RefreshInterval) * time.Second)
		go func() {
			for {
				select {
				case <-ticker.C:
					b.Refresh()
				case <-quitCh:
					log.Infof("Quit service plugin")
					ticker.Stop()
					return
				}
			}
		}()
	}

	return b, quitCh, nil
}
