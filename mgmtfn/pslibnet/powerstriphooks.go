package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/contiv/netplugin/core"
	"github.com/mapuri/libnetwork/driverapi"

	log "github.com/Sirupsen/logrus"
)

type clientRequest struct {
	Method  string `json:"Method"`
	Request string `json:"Request"`
	Body    string `json:"Body"`
}

type serverResponse struct {
	ContentType string `json:"ContentType"`
	Body        string `json:"Body"`
	Code        int    `json:"Code"`
}

type powerStripRequest struct {
	PowerstripProtocolVersion int            `json:"PowerstripProtocolVersion"`
	Type                      string         `json:"Type"`
	ClientRequest             clientRequest  `json:"ClientRequest"`
	ServerResponse            serverResponse `json:"ServerResponse"`
}

type powerStripResponse struct {
	PowerstripProtocolVersion int            `json:"PowerstripProtocolVersion"`
	ModifiedClientRequest     clientRequest  `json:"ModifiedClientRequest"`
	ModifiedServerResponse    serverResponse `json:"ModifiedServerResponse"`
}

type containerNet struct {
	tenantID string
	netID    string
}

type handlerFunc func(*powerStripRequest) (*powerStripResponse, error)

// PwrStrpAdptr keeps state based on requests and responses as seen by the adapter
type PwrStrpAdptr struct {
	driver driverapi.Driver
	//track the network that a container belongs to
	containerNets map[string][]containerNet
	// tracks the network received in last container create. Set on
	// getting contianer pre-create and cleared on getting post-create.
	outstandingNet containerNet
	// tracks the container-id/name received in last container start. Set on
	// getting contianer pre-start and cleared on getting post-start.
	outstandingContID string
	// track name to ID mapping when a conatiner name was specified
	nameIDMap map[string]string
	// hooks implememted by this adapter
	powerstripHooks map[string]handlerFunc
}

func makeClientRequest(req *powerStripRequest) *powerStripResponse {
	return &powerStripResponse{
		PowerstripProtocolVersion: req.PowerstripProtocolVersion,
		ModifiedClientRequest: clientRequest{
			Method:  req.ClientRequest.Method,
			Request: req.ClientRequest.Request,
			Body:    req.ClientRequest.Body,
		},
	}
}

func makeServerResponse(req *powerStripRequest) *powerStripResponse {
	return &powerStripResponse{
		PowerstripProtocolVersion: req.PowerstripProtocolVersion,
		ModifiedServerResponse: serverResponse{
			ContentType: req.ServerResponse.ContentType,
			Body:        req.ServerResponse.Body,
			Code:        req.ServerResponse.Code,
		},
	}
}

// Init initializes an instance of PwrStrpAdptr
func (adptr *PwrStrpAdptr) Init(d driverapi.Driver) error {
	adptr.driver = d
	adptr.containerNets = make(map[string][]containerNet)
	adptr.outstandingNet = containerNet{"", ""}
	adptr.outstandingContID = ""
	adptr.nameIDMap = make(map[string]string)
	adptr.powerstripHooks = map[string]handlerFunc{
		"pre-hook:create":  adptr.handlePreCreate,
		"post-hook:create": adptr.handlePostCreate,
		"pre-hook:start":   adptr.handlePreStart,
		"post-hook:start":  adptr.handlePostStart,
		"pre-hook:stop":    adptr.handlePreStop,
		"pre-hook:delete":  adptr.handlePreDelete,
	}

	return nil
}

func extractHookStr(req *powerStripRequest) string {

	str := ""
	if req.ClientRequest.Method == "DELETE" {
		str = fmt.Sprintf("%s:delete", req.Type)
	} else {
		str = fmt.Sprintf("%s:%s", req.Type,
			req.ClientRequest.Request[strings.LastIndex(req.ClientRequest.Request, "/")+1:])
	}
	//remove any query parameters
	if strings.Index(str, "?") != -1 {
		str = string(str[:strings.Index(str, "?")])
	}

	return str
}

// CallHook handles the calls from powerstrip adapter. In absence of formal plugin hooks,
// the netplugin hooks into post-create and pre-delete requests from power strip.
func (adptr *PwrStrpAdptr) CallHook(w http.ResponseWriter, r *http.Request) {
	log.Printf("handling new request")
	req := &powerStripRequest{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(req)
	if err != nil {
		log.Printf("failed to parse powerstrip request. Error: %s", err)
		return
	}
	log.Printf("powerstrip request: %+v", req)

	fn, ok := adptr.powerstripHooks[extractHookStr(req)]
	if !ok {
		log.Printf("No handler for hook %q, request: %+v", extractHookStr(req), req)
		http.Error(w, "Unhandled request", http.StatusInternalServerError)
		return
	}

	resp, err := fn(req)
	if err != nil {
		log.Printf("failed to handle request. Error: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	err = encoder.Encode(resp)
	if err != nil {
		log.Printf("failed to write response. Error: %s", err)
	}
	return
}

// Check that container is created with a 'netid' label. This helps us map the
// container to a network. This label could be put by one of docker clients (cli,
// compose etc)
func (adptr *PwrStrpAdptr) handlePreCreate(req *powerStripRequest) (*powerStripResponse, error) {
	//structure of interesting fields in the create request
	type DockerCreateRequest struct {
		Labels map[string]string `json:"Labels"`
	}

	dockerReq := &DockerCreateRequest{}
	err := json.Unmarshal([]byte(req.ClientRequest.Body), dockerReq)
	if err != nil {
		return nil, err
	}

	if netID, ok := dockerReq.Labels["netid"]; !ok || netID == "" {
		return nil, core.Errorf("Container doesn't contain a valid 'netid' label. Labels: %+v Body: %q",
			dockerReq, req.ClientRequest.Body)
	} else if tenantID, ok := dockerReq.Labels["tenantid"]; !ok || tenantID == "" {
		return nil, core.Errorf("Container doesn't contain a valid 'tenantid' label. Labels: %+v Body: %q",
			dockerReq, req.ClientRequest.Body)
	} else {
		// XXX: record the outstanding network for which we are yet to receive a
		// corresponding container-id (in server response). This simplifies things
		// by assuming that there can't be two outstanding request. Need to revisit
		// and handle this correctly.
		adptr.outstandingNet = containerNet{tenantID: tenantID, netID: netID}
	}

	return makeClientRequest(req), nil
}

// Map the 'netid' received in create request to the container-id
func (adptr *PwrStrpAdptr) handlePostCreate(req *powerStripRequest) (*powerStripResponse, error) {
	defer func() { adptr.outstandingNet = containerNet{"", ""} }()

	// ignore the response if container create failed
	if req.ServerResponse.Code != 201 {
		return makeServerResponse(req), nil
	}

	// should not happen
	if adptr.outstandingNet.netID == "" {
		return nil, core.Errorf("received a container create response, without corresponding create!")
	}

	//structure of interesting fields in the create response
	type DockerCreateResponse struct {
		ContID string `json:"Id"`
	}
	dockerResp := &DockerCreateResponse{}
	err := json.Unmarshal([]byte(req.ServerResponse.Body), dockerResp)
	if err != nil {
		return nil, err
	}

	//update the adptr to remember the netid to container-id mapping
	if _, ok := adptr.containerNets[dockerResp.ContID]; !ok {
		adptr.containerNets[dockerResp.ContID] = make([]containerNet, 0)
	}
	adptr.containerNets[dockerResp.ContID] = append(adptr.containerNets[dockerResp.ContID],
		adptr.outstandingNet)
	// if a container name was specified update that mapping as well
	if strings.Index(req.ClientRequest.Request, "?") != -1 {
		queryParam := string(req.ClientRequest.Request[strings.Index(req.ClientRequest.Request, "?")+1:])
		// right now create API just takes name as query parameter
		name := strings.Split(queryParam, "=")[1]
		adptr.nameIDMap[name] = dockerResp.ContID
	}

	return makeServerResponse(req), nil
}

func extractContIDOrName(req clientRequest) string {
	reqParts := strings.Split(req.Request, "/")
	if req.Method == "POST" {
		// container-id is at last but one position in the request string for
		// POST requests (i.e. start and stop)
		return reqParts[len(reqParts)-2]
	}
	// container-id is at last position in the request string for DELETE
	// requests
	return reqParts[len(reqParts)-1]
}

// take conatiner ID (complete or short hash) or name and return full container
// ID if container exists, else return empty string
func (adptr *PwrStrpAdptr) getFullContainerID(contIDOrName string) string {
	// see if caller passed a full ID
	if _, ok := adptr.containerNets[contIDOrName]; ok {
		return contIDOrName
	}
	// see if caller passed a name
	if id, ok := adptr.nameIDMap[contIDOrName]; ok {
		return id
	}
	//see if caller passed a short conatiner ID
	retID := ""
	for id := range adptr.containerNets {
		if strings.HasPrefix(id, contIDOrName) && retID == "" {
			retID = id
		} else if retID != "" {
			// found overlapping containers with the passed prefix
			return ""
		}
	}
	return retID
}

// Record the container-id, so that we can create endpoints for the container
// once it is started (and we get post-start) trigger
func (adptr *PwrStrpAdptr) handlePreStart(req *powerStripRequest) (*powerStripResponse, error) {
	contIDOrName := extractContIDOrName(req.ClientRequest)
	if _, ok := adptr.containerNets[adptr.getFullContainerID(contIDOrName)]; !ok {
		return nil, core.Errorf("got a start request for non existent container. contIDOrName: %s Request: %+v",
			contIDOrName, req)
	}
	adptr.outstandingContID = adptr.getFullContainerID(contIDOrName)

	return makeClientRequest(req), nil
}

// Call the network driver's CreateEndpoint API for all networks that the
// container belongs to.
func (adptr *PwrStrpAdptr) handlePostStart(req *powerStripRequest) (*powerStripResponse, error) {
	defer func() { adptr.outstandingContID = "" }()

	// ignore the response if container start failed
	if req.ServerResponse.Code != 204 {
		return makeServerResponse(req), nil
	}

	// should not happen
	if adptr.outstandingContID == "" {
		return nil, core.Errorf("received a container start response, without corresponding start request!")
	}

	// should not happen
	if _, ok := adptr.containerNets[adptr.outstandingContID]; !ok {
		return nil, core.Errorf("received a container start response for unknown container %s",
			adptr.outstandingContID)
	}

	// Now create an endpoint for every network this container is part of
	contID := adptr.outstandingContID
	for _, net := range adptr.containerNets[contID] {
		// in libnetwork netUUID and epUUID are derived by the libnetwork,
		// just deriving a unique string for now.
		netUUID := driverapi.UUID(fmt.Sprintf("%s-%s", net.tenantID, net.netID))
		epUUID := driverapi.UUID(fmt.Sprintf("%s-%s-%s", net.tenantID, net.netID, contID))
		_, err := adptr.driver.CreateEndpoint(netUUID, epUUID, "",
			DriverConfig{net.tenantID, net.netID, contID})
		defer func(netUUID, epUUID driverapi.UUID) {
			if err != nil {
				adptr.driver.DeleteEndpoint(netUUID, epUUID)
			}
		}(netUUID, epUUID)
		if err != nil {
			return nil, core.Errorf("Failed to create endpoint for net: %+v container: %q with net(s): %+v. Error: %s",
				net, contID, adptr.containerNets[contID], err)
		}
	}

	return makeServerResponse(req), nil
}

// Call the network driver's DeleteEndpoint API for all the networks that the
// container belongs to.
func (adptr *PwrStrpAdptr) handlePreStop(req *powerStripRequest) (*powerStripResponse, error) {
	contIDOrName := extractContIDOrName(req.ClientRequest)
	fullContID := adptr.getFullContainerID(contIDOrName)
	if _, ok := adptr.containerNets[fullContID]; !ok {
		log.Printf("got a stop request for non existent container. contIDOrName: %s Request: %+v",
			contIDOrName, req)
		// let the request be forwarded to docker
	} else {
		// delete the endpoint for every network this container is part of
		for _, net := range adptr.containerNets[fullContID] {
			// in libnetwork netUUID and epUUID are derived by the libnetwork,
			// just deriving a unique string for now.
			netUUID := driverapi.UUID(fmt.Sprintf("%s-%s", net.tenantID, net.netID))
			epUUID := driverapi.UUID(fmt.Sprintf("%s-%s-%s", net.tenantID, net.netID, fullContID))
			err := adptr.driver.DeleteEndpoint(netUUID, epUUID)
			if err != nil {
				log.Printf("Failed to delete endpoint for net: %+v container: %q with net(s): %+v. Error: %s",
					net, contIDOrName, adptr.containerNets[fullContID], err)
				continue
			}
		}
	}

	return makeClientRequest(req), nil
}

// Clear the mapping of container's uuid to all networks
func (adptr *PwrStrpAdptr) handlePreDelete(req *powerStripRequest) (*powerStripResponse, error) {
	contIDOrName := extractContIDOrName(req.ClientRequest)
	fullContID := adptr.getFullContainerID(contIDOrName)
	if _, ok := adptr.containerNets[fullContID]; !ok {
		log.Printf("got a delete request for non existent container. contIDOrName: %s Request: %+v",
			contIDOrName, req)
		// let the request be forwarded to docker
	} else {
		// XXX: with powerstrip there is no way to stimulate a stop request
		// if a container exits, so perform the stop related cleanup on delete as well
		adptr.handlePreStop(req)
		delete(adptr.containerNets, fullContID)
		delete(adptr.nameIDMap, contIDOrName)
	}

	return makeClientRequest(req), nil
}
