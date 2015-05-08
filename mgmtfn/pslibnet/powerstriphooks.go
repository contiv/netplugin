package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/mapuri/libnetwork/driverapi"
)

type ClientRequest struct {
	Method  string `json:"Method"`
	Request string `json:"Request"`
	Body    string `json:"Body"`
}

type ServerResponse struct {
	ContentType string `json:"ContentType"`
	Body        string `json:"Body"`
	Code        int    `json:"Code"`
}

type PowerStripRequest struct {
	PowerstripProtocolVersion int            `json:"PowerstripProtocolVersion"`
	Type                      string         `json:"Type"`
	ClientRequest             ClientRequest  `json:"ClientRequest"`
	ServerResponse            ServerResponse `json:"ServerResponse"`
}

type PowerStripResponse struct {
	PowerstripProtocolVersion int            `json:"PowerstripProtocolVersion"`
	ModifiedClientRequest     ClientRequest  `json:"ModifiedClientRequest"`
	ModifiedServerResponse    ServerResponse `json:"ModifiedServerResponse"`
}

type ContainerNet struct {
	tenantId string
	netId    string
}

type handlerFunc func(*PowerStripRequest) (*PowerStripResponse, error)

//structure to keep state based on requests and responses as seen by the adapter
type PwrStrpAdptr struct {
	driver driverapi.Driver
	//track the network that a container belongs to
	containerNets map[string][]ContainerNet
	// tracks the network received in last container create. Set on
	// getting contianer pre-create and cleared on getting post-create.
	outstandingNet ContainerNet
	// tracks the container-id/name received in last container start. Set on
	// getting contianer pre-start and cleared on getting post-start.
	outstandingContId string
	// track name to Id mapping when a conatiner name was specified
	nameIdMap map[string]string
	// hooks implememted by this adapter
	powerstripHooks map[string]handlerFunc
}

func makeClientRequest(req *PowerStripRequest) *PowerStripResponse {
	return &PowerStripResponse{
		PowerstripProtocolVersion: req.PowerstripProtocolVersion,
		ModifiedClientRequest: ClientRequest{
			Method:  req.ClientRequest.Method,
			Request: req.ClientRequest.Request,
			Body:    req.ClientRequest.Body,
		},
	}
}

func makeServerResponse(req *PowerStripRequest) *PowerStripResponse {
	return &PowerStripResponse{
		PowerstripProtocolVersion: req.PowerstripProtocolVersion,
		ModifiedServerResponse: ServerResponse{
			ContentType: req.ServerResponse.ContentType,
			Body:        req.ServerResponse.Body,
			Code:        req.ServerResponse.Code,
		},
	}
}

func (adptr *PwrStrpAdptr) Init(d driverapi.Driver) error {
	adptr.driver = d
	adptr.containerNets = make(map[string][]ContainerNet)
	adptr.outstandingNet = ContainerNet{"", ""}
	adptr.outstandingContId = ""
	adptr.nameIdMap = make(map[string]string)
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

func extractHookStr(req *PowerStripRequest) string {

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

// handle the calls from powerstrip adapter. In absence of formal plugin hooks,
// the netplugin hooks into post-create and pre-delete requests from power strip.
func (adptr *PwrStrpAdptr) CallHook(w http.ResponseWriter, r *http.Request) {
	log.Printf("handling new request")
	req := &PowerStripRequest{}
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
func (adptr *PwrStrpAdptr) handlePreCreate(req *PowerStripRequest) (*PowerStripResponse, error) {
	//structure of interesting fields in the create request
	type DockerCreateRequest struct {
		Labels map[string]string `json:"Labels"`
	}

	dockerReq := &DockerCreateRequest{}
	err := json.Unmarshal([]byte(req.ClientRequest.Body), dockerReq)
	if err != nil {
		return nil, err
	}

	if netId, ok := dockerReq.Labels["netid"]; !ok || netId == "" {
		return nil, fmt.Errorf("Container doesn't contain a valid 'netid' label. Labels: %+v Body: %q",
			dockerReq, req.ClientRequest.Body)
	} else if tenantId, ok := dockerReq.Labels["tenantid"]; !ok || tenantId == "" {
		return nil, fmt.Errorf("Container doesn't contain a valid 'tenantid' label. Labels: %+v Body: %q",
			dockerReq, req.ClientRequest.Body)
	} else {
		// XXX: record the outstanding network for which we are yet to receive a
		// corresponding container-id (in server response). This simplifies things
		// by assuming that there can't be two outstanding request. Need to revisit
		// and handle this correctly.
		adptr.outstandingNet = ContainerNet{tenantId: tenantId, netId: netId}
	}

	return makeClientRequest(req), nil
}

// Map the 'netid' received in create request to the container-id
func (adptr *PwrStrpAdptr) handlePostCreate(req *PowerStripRequest) (*PowerStripResponse, error) {
	defer func() { adptr.outstandingNet = ContainerNet{"", ""} }()

	// ignore the response if container create failed
	if req.ServerResponse.Code != 201 {
		return makeServerResponse(req), nil
	}

	// should not happen
	if adptr.outstandingNet.netId == "" {
		return nil, fmt.Errorf("received a container create response, without corresponding create!")
	}

	//structure of interesting fields in the create response
	type DockerCreateResponse struct {
		ContId string `json:"Id"`
	}
	dockerResp := &DockerCreateResponse{}
	err := json.Unmarshal([]byte(req.ServerResponse.Body), dockerResp)
	if err != nil {
		return nil, err
	}

	//update the adptr to remember the netid to container-id mapping
	if _, ok := adptr.containerNets[dockerResp.ContId]; !ok {
		adptr.containerNets[dockerResp.ContId] = make([]ContainerNet, 0)
	}
	adptr.containerNets[dockerResp.ContId] = append(adptr.containerNets[dockerResp.ContId],
		adptr.outstandingNet)
	// if a container name was specified update that mapping as well
	if strings.Index(req.ClientRequest.Request, "?") != -1 {
		queryParam := string(req.ClientRequest.Request[strings.Index(req.ClientRequest.Request, "?")+1:])
		// right now create API just takes name as query parameter
		name := strings.Split(queryParam, "=")[1]
		adptr.nameIdMap[name] = dockerResp.ContId
	}

	return makeServerResponse(req), nil
}

func extractContIdOrName(req ClientRequest) string {
	reqParts := strings.Split(req.Request, "/")
	if req.Method == "POST" {
		// container-id is at last but one position in the request string for
		// POST requests (i.e. start and stop)
		return reqParts[len(reqParts)-2]
	} else {
		// container-id is at last position in the request string for DELETE
		// requests
		return reqParts[len(reqParts)-1]
	}
}

// take conatiner Id (complete or short hash) or name and return full container
// Id if container exists, else return empty string
func (adptr *PwrStrpAdptr) getFullContainerId(contIdOrName string) string {
	// see if caller passed a full Id
	if _, ok := adptr.containerNets[contIdOrName]; ok {
		return contIdOrName
	}
	// see if caller passed a name
	if id, ok := adptr.nameIdMap[contIdOrName]; ok {
		return id
	}
	//see if caller passed a short conatiner Id
	retId := ""
	for id, _ := range adptr.containerNets {
		if strings.HasPrefix(id, contIdOrName) && retId == "" {
			retId = id
		} else if retId != "" {
			// found overlapping containers with the passed prefix
			return ""
		}
	}
	return retId
}

// Record the container-id, so that we can create endpoints for the container
// once it is started (and we get post-start) trigger
func (adptr *PwrStrpAdptr) handlePreStart(req *PowerStripRequest) (*PowerStripResponse, error) {
	contIdOrName := extractContIdOrName(req.ClientRequest)
	if _, ok := adptr.containerNets[adptr.getFullContainerId(contIdOrName)]; !ok {
		return nil, fmt.Errorf("got a start request for non existent container. contIdOrName: %s Request: %+v",
			contIdOrName, req)
	} else {
		adptr.outstandingContId = adptr.getFullContainerId(contIdOrName)
	}

	return makeClientRequest(req), nil
}

// Call the network driver's CreateEndpoint API for all networks that the
// container belongs to.
func (adptr *PwrStrpAdptr) handlePostStart(req *PowerStripRequest) (*PowerStripResponse, error) {
	defer func() { adptr.outstandingContId = "" }()

	// ignore the response if container start failed
	if req.ServerResponse.Code != 204 {
		return makeServerResponse(req), nil
	}

	// should not happen
	if adptr.outstandingContId == "" {
		return nil, fmt.Errorf("received a container start response, without corresponding start request!")
	}

	// should not happen
	if _, ok := adptr.containerNets[adptr.outstandingContId]; !ok {
		return nil, fmt.Errorf("received a container start response for unknown container %s",
			adptr.outstandingContId)
	}

	// Now create an endpoint for every network this container is part of
	contId := adptr.outstandingContId
	for _, net := range adptr.containerNets[contId] {
		// in libnetwork netUUID and epUUID are derived by the libnetwork,
		// just deriving a unique string for now.
		netUuid := driverapi.UUID(fmt.Sprintf("%s-%s", net.tenantId, net.netId))
		epUuid := driverapi.UUID(fmt.Sprintf("%s-%s-%s", net.tenantId, net.netId, contId))
		_, err := adptr.driver.CreateEndpoint(netUuid, epUuid, "",
			DriverConfig{net.tenantId, net.netId, contId})
		defer func(netUuid, epUuid driverapi.UUID) {
			if err != nil {
				adptr.driver.DeleteEndpoint(netUuid, epUuid)
			}
		}(netUuid, epUuid)
		if err != nil {
			return nil, fmt.Errorf("Failed to create endpoint for net: %+v container: %q with net(s): %+v. Error: %s",
				net, contId, adptr.containerNets[contId], err)
		}
	}

	return makeServerResponse(req), nil
}

// Call the network driver's DeleteEndpoint API for all the networks that the
// container belongs to.
func (adptr *PwrStrpAdptr) handlePreStop(req *PowerStripRequest) (*PowerStripResponse, error) {
	contIdOrName := extractContIdOrName(req.ClientRequest)
	fullContId := adptr.getFullContainerId(contIdOrName)
	if _, ok := adptr.containerNets[fullContId]; !ok {
		log.Printf("got a stop request for non existent container. contIdOrName: %s Request: %+v",
			contIdOrName, req)
		// let the request be forwarded to docker
	} else {
		// delete the endpoint for every network this container is part of
		for _, net := range adptr.containerNets[fullContId] {
			// in libnetwork netUUID and epUUID are derived by the libnetwork,
			// just deriving a unique string for now.
			netUuid := driverapi.UUID(fmt.Sprintf("%s-%s", net.tenantId, net.netId))
			epUuid := driverapi.UUID(fmt.Sprintf("%s-%s-%s", net.tenantId, net.netId, fullContId))
			err := adptr.driver.DeleteEndpoint(netUuid, epUuid)
			if err != nil {
				log.Printf("Failed to delete endpoint for net: %+v container: %q with net(s): %+v. Error: %s",
					net, contIdOrName, adptr.containerNets[fullContId], err)
				continue
			}
		}
	}

	return makeClientRequest(req), nil
}

// Clear the mapping of container's uuid to all networks
func (adptr *PwrStrpAdptr) handlePreDelete(req *PowerStripRequest) (*PowerStripResponse, error) {
	contIdOrName := extractContIdOrName(req.ClientRequest)
	fullContId := adptr.getFullContainerId(contIdOrName)
	if _, ok := adptr.containerNets[fullContId]; !ok {
		log.Printf("got a delete request for non existent container. contIdOrName: %s Request: %+v",
			contIdOrName, req)
		// let the request be forwarded to docker
	} else {
		// XXX: with powerstrip there is no way to stimulate a stop request
		// if a container exits, so perform the stop related cleanup on delete as well
		adptr.handlePreStop(req)
		delete(adptr.containerNets, fullContId)
		delete(adptr.nameIdMap, contIdOrName)
	}

	return makeClientRequest(req), nil
}
