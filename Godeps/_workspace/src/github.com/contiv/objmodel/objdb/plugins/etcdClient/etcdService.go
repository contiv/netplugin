package etcdClient

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	api "github.com/contiv/objmodel/objdb"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/go-etcd/etcd"
)

const SERVICE_TTL = 60

// Service state
type serviceState struct {
	ServiceName string // Name of the service
	HostAddr    string // Host name or IP address where its running
	Port        int    // Port number where its listening

	// Channel to stop ttl refresh
	stopChan chan bool
}

// Register a service
// Service is registered with a ttl for 60sec and a goroutine is created
// to refresh the ttl.
func (self *EtcdPlugin) RegisterService(serviceInfo api.ServiceInfo) error {
	keyName := "/contiv.io/service/" + serviceInfo.ServiceName + "/" +
		serviceInfo.HostAddr + ":" + strconv.Itoa(serviceInfo.Port)

	log.Infof("Registering service key: %s, value: %+v", keyName, serviceInfo)

	// JSON format the object
	jsonVal, err := json.Marshal(serviceInfo)
	if err != nil {
		log.Errorf("Json conversion error. Err %v", err)
		return err
	}

	// Set it via etcd client
	_, err = self.client.Set(keyName, string(jsonVal[:]), SERVICE_TTL)
	if err != nil {
		log.Errorf("Error setting key %s, Err: %v", keyName, err)
		return err
	}

	// Run refresh in background
	stopChan := make(chan bool, 1)
	go refreshService(self.client, keyName, string(jsonVal[:]), stopChan)

	// Store it in DB
	self.serviceDb[keyName] = &serviceState{
		ServiceName: serviceInfo.ServiceName,
		HostAddr:    serviceInfo.HostAddr,
		Port:        serviceInfo.Port,
		stopChan:    stopChan,
	}

	return nil
}

// List all end points for a service
func (self *EtcdPlugin) GetService(name string) ([]api.ServiceInfo, error) {
	keyName := "/contiv.io/service/" + name + "/"

	// Get the object from etcd client
	resp, err := self.client.Get(keyName, true, true)
	if err != nil {
		if strings.Contains(err.Error(), "Key not found") {
			return nil, nil
		} else {
			log.Errorf("Error getting key %s. Err: %v", keyName, err)
			return nil, err
		}

	}

	if !resp.Node.Dir {
		log.Errorf("Err. Response is not a directory: %+v", resp.Node)
		return nil, errors.New("Invalid Response from etcd")
	}

	srvcList := make([]api.ServiceInfo, 0)

	// Parse each node in the directory
	for _, node := range resp.Node.Nodes {
		var respSrvc api.ServiceInfo
		// Parse JSON response
		err = json.Unmarshal([]byte(node.Value), &respSrvc)
		if err != nil {
			log.Errorf("Error parsing object %s, Err %v", node.Value, err)
			return nil, err
		}

		srvcList = append(srvcList, respSrvc)
	}

	return srvcList, nil
}

// Watch for a service
func (self *EtcdPlugin) WatchService(name string,
	eventCh chan api.WatchServiceEvent, stopCh chan bool) error {
	keyName := "/contiv.io/service/" + name + "/"

	// Create channels
	watchCh := make(chan *etcd.Response, 1)
	watchStopCh := make(chan bool, 1)

	// Start the watch thread
	go func() {
		log.Infof("Watching for service: %s", keyName)
		// Start the watch
		_, err := self.client.Watch(keyName, 0, true, watchCh, watchStopCh)
		if (err != nil) && (err != etcd.ErrWatchStoppedByUser) {
			log.Errorf("Error watching service %s. Err: %v", keyName, err)

			// Emit the event
			eventCh <- api.WatchServiceEvent{EventType: api.WatchServiceEventError}
		}
		log.Infof("Watch thread exiting..")
	}()

	// handle messages from watch service
	go func() {
		for {
			select {
			case watchResp := <-watchCh:
				log.Debugf("Received event %#v\n Node: %#v", watchResp, watchResp.Node)

				// derive service info from key
				srvKey := strings.TrimPrefix(watchResp.Node.Key, "/contiv.io/service/")
				srvName := strings.Split(srvKey, "/")[0]
				hostInfo := strings.Split(srvKey, "/")[1]
				hostAddr := strings.Split(hostInfo, ":")[0]
				portNum, _ := strconv.Atoi(strings.Split(hostInfo, ":")[1])

				// Build service info
				srvInfo := api.ServiceInfo{
					ServiceName: srvName,
					HostAddr:    hostAddr,
					Port:        portNum,
				}

				// We ignore all events except Set/Delete/Expire
				// Note that Set event doesnt exactly mean new service end point.
				// If a service restarts and re-registers before it expired, we'll
				// receive set again. receivers need to handle this case
				if watchResp.Action == "set" {
					log.Infof("Sending service add event: %+v", srvInfo)
					// Send Add event
					eventCh <- api.WatchServiceEvent{
						EventType:   api.WatchServiceEventAdd,
						ServiceInfo: srvInfo,
					}
				} else if (watchResp.Action == "delete") ||
					(watchResp.Action == "expire") {

					log.Infof("Sending service del event: %+v", srvInfo)

					// Send Delete event
					eventCh <- api.WatchServiceEvent{
						EventType:   api.WatchServiceEventDel,
						ServiceInfo: srvInfo,
					}
				}
			case stopReq := <-stopCh:
				if stopReq {
					// Stop watch and return
					log.Infof("Stopping watch on %s", keyName)
					watchStopCh <- true
					return
				}
			}
		}
	}()

	return nil
}

// Deregister a service
// This removes the service from the registry and stops the refresh groutine
func (self *EtcdPlugin) DeregisterService(serviceInfo api.ServiceInfo) error {
	keyName := "/contiv.io/service/" + serviceInfo.ServiceName + "/" +
		serviceInfo.HostAddr + ":" + strconv.Itoa(serviceInfo.Port)

	// Find it in the database
	srvState := self.serviceDb[keyName]
	if srvState == nil {
		log.Errorf("Could not find the service in db %s", keyName)
		return errors.New("Service not found")
	}

	// stop the refresh thread and delete service
	srvState.stopChan <- true
	delete(self.serviceDb, keyName)

	// Delete the service instance
	_, err := self.client.Delete(keyName, false)
	if err != nil {
		log.Errorf("Error getting key %s. Err: %v", keyName, err)
		return err
	}

	return nil
}

// Keep refreshing the service every 30sec
func refreshService(client *etcd.Client, keyName string, keyVal string, stopChan chan bool) {
	for {
		select {
		case <-time.After(time.Second * time.Duration(SERVICE_TTL/3)):
			log.Debugf("Refreshing key: %s", keyName)

			_, err := client.Update(keyName, keyVal, SERVICE_TTL)
			if err != nil {
				log.Errorf("Error updating key %s, Err: %v", keyName, err)
			}

		case <-stopChan:
			log.Infof("Stop refreshing key: %s", keyName)
			return
		}
	}
}
