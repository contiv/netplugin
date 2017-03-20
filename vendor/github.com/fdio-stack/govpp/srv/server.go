// Generates Go bindings for all VPP APIs located in the json directory.
// go:generate binapi_generator --input-dir=json --output-dir=go

package srv

import (
	"errors"
	"fmt"
	"govpp-master/examples/go/interfaces"

	log "github.com/Sirupsen/logrus"
	"github.com/fdio-stack/govpp"
	"github.com/fdio-stack/govpp/api"
	"github.com/fdio-stack/govpp/messages/go/acl"
	"github.com/fdio-stack/govpp/messages/go/vpe"
)

type vppBridgeDomain struct {
	name         string
	bridgeID     uint32
	hasInterface bool
}

type vppInterface struct {
	name      string
	swIfIndex uint32
	adminUp   uint8
	ipAddr    string
}

type vppRuleT struct {
	index uint32
}

// Start with bridgedainID = 1
var nextBdid uint32 = 1

// Keeps a map of the associated Contiv Network ID and VPP bridge domains
var vppBridgeByID = make(map[string]*vppBridgeDomain)
var vppIntfByName = make(map[string]*vppInterface)
var vppRuleByID = make(map[string]*vppRuleT)

/*
 ***************************************************************

 *** PUBLIC functions

 ***************************************************************
 */

// VppConnect export the VPP connect function to the public
func VppConnect() {
	vpp_connect()
}

// VppAddDelBridgeDomain creates a bridge domain inside VPP
func VppAddDelBridgeDomain(id string, isAdd bool) (uint32, error) {
	if isAdd {
		bdid := nextBdid
		vppBridge := vppBridgeDomain{
			id, bdid, false}
		err := vpp_add_del_l2_bridge_domain(bdid, 1)
		if err != nil {
			return 0, err
		}
		vppBridgeByID[id] = &vppBridge
		nextBdid++
		return bdid, nil
	}
	bdid := nextBdid - 1
	delete(vppBridgeByID, id)
	err := vpp_add_del_l2_bridge_domain(bdid, 0)
	if err != nil {
		return 0, err
	}
	nextBdid--
	return bdid, nil
}

// VppAddInterface creates an af_packet interface in VPP
func VppAddInterface(vppIntf string) error {
	err := vpp_add_af_packet_interface(vppIntf)
	if err != nil {
		return err
	}
	return nil
}

// VppInterfaceAdminUp sets interface flags state up
func VppInterfaceAdminUp(vppIntf string) error {
	err := vpp_set_vpp_interface_adminup(vppIntf)
	if err != nil {
		return err
	}
	return nil
}

// VppSetInterfaceL2Bridge requests bridge mode for interface
func VppSetInterfaceL2Bridge(id string, vppIntf string) error {
	err := vpp_set_interface_l2_bridge(id, vppIntf)
	if err != nil {
		return err
	}
	return nil
}

// VppACLAddReplaceRule adds/replaces a rule in VPP
func VppACLAddReplaceRule(vppRule *acl.ACLRule) error {
	err := vpp_acl_add_replace_rule(vppRule)
	if err != nil {
		return err
	}
	return nil
}

// VppACLDelRule deletes an ACL Rule in vpp
func VppACLDelRule(vppRule *acl.ACLRule) error {
	err := vpp_acl_del_rule(vppRule)
	if err != nil {
		return err
	}
	return nil
}

/*
 ***************************************************************

 *** VPP Connect / Disconnect

 ***************************************************************
 */

func vpp_connect() {
	log.Infof("Connected to VPP driver")
}

/*
 ***************************************************************

 *** VPP BRIDGE DOMAIN

 ***************************************************************
 */

func vpp_add_del_l2_bridge_domain(bdid uint32, isAdd uint8) error {

	conn := govpp.Connect()
	defer conn.Disconnect()

	ch := conn.NewApiChannel()
	defer ch.Close()

	req := &vpe.BridgeDomainAddDel{
		BdID:    bdid,
		Flood:   1,
		UuFlood: 1,
		Forward: 1,
		Learn:   1,
		ArpTerm: 1,
		MacAge:  0,
		IsAdd:   isAdd,
	}
	// brecode - change to argument values

	// send the request - channel API instead of SendRequest
	ch.ReqChan <- &api.VppRequest{Message: req}

	// receive the response - channel API instead of ReceiveReply
	vppReply := <-ch.ReplyChan
	reply := &vpe.BridgeDomainAddDelReply{}
	ch.Decoder.DecodeMsg(vppReply.Data, reply)

	if reply.Retval != 0 {
		return errors.New("Could not add/del bridge domain")
	}

	return nil
}

func vpp_set_interface_l2_bridge(id string, vppIntf string) error {
	conn := govpp.Connect()
	defer conn.Disconnect()

	ch := conn.NewApiChannel()
	defer ch.Close()

	_, ok := vppBridgeByID[id]
	if !ok {
		return errors.New("govpp: vpp_set_interface_l2_bridge: ID not found in vppBridgeByID")
	}
	_, ok = vppIntfByName[vppIntf]
	if !ok {
		return errors.New("Interface not found in vppIntfByName")
	}

	req := &vpe.SwInterfaceSetL2Bridge{
		RxSwIfIndex: vppIntfByName[vppIntf].swIfIndex,
		BdID:        vppBridgeByID[id].bridgeID,
		Shg:         0,
		Bvi:         0,
		Enable:      1,
	}

	// send the request - channel API instead of SendRequest
	ch.ReqChan <- &api.VppRequest{Message: req}

	// receive the response - channel API instead of ReceiveReply
	vppReply := <-ch.ReplyChan
	reply := &vpe.SwInterfaceSetL2BridgeReply{}
	ch.Decoder.DecodeMsg(vppReply.Data, reply)

	if reply.Retval != 0 {
		return errors.New("Could not set bridge mode for interface")
	}
	return nil
}

/*
 ***************************************************************

 *** VPP Interface Add / Del, Set Flags

 ***************************************************************
 */

func vpp_add_af_packet_interface(vppIntf string) error {
	conn := govpp.Connect()
	defer conn.Disconnect()

	ch := conn.NewApiChannel()
	defer ch.Close()

	req := &vpe.AfPacketCreate{
		HostIfName:      []byte(vppIntf),
		UseRandomHwAddr: 1,
	}

	// send the request - channel API instead of SendRequest
	ch.ReqChan <- &api.VppRequest{Message: req}

	// receive the response - channel API instead of ReceiveReply
	vppReply := <-ch.ReplyChan
	reply := &vpe.AfPacketCreateReply{}
	ch.Decoder.DecodeMsg(vppReply.Data, reply)

	if reply.Retval != 0 {
		return errors.New("Could not add ad_packet interface")
	}

	vppInt := vppInterface{
		vppIntf,
		reply.SwIfIndex,
		0,
		""}
	vppIntfByName[vppIntf] = &vppInt
	return nil
}

func vpp_set_vpp_interface_adminup(vppIntf string) error {
	conn := govpp.Connect()
	defer conn.Disconnect()

	ch := conn.NewApiChannel()
	defer ch.Close()

	_, ok := vppIntfByName[vppIntf]
	if !ok {
		return errors.New("Interface not found in vppIntfByName")
	}

	req := &interfaces.SwInterfaceSetFlags{
		SwIfIndex:   vppIntfByName[vppIntf].swIfIndex,
		AdminUpDown: 1,
	}

	// send the request - channel API instead of SendRequest
	ch.ReqChan <- &api.VppRequest{Message: req}

	// receive the response - channel API instead of ReceiveReply
	vppReply := <-ch.ReplyChan
	reply := &interfaces.SwInterfaceSetFlagsReply{}
	ch.Decoder.DecodeMsg(vppReply.Data, reply)

	if reply.Retval != 0 {
		return errors.New("Could not add set af_packet interface flag, admin state up")
	}
	return nil
}

/*
 ***************************************************************

 *** VPP ACL

 ***************************************************************
 */

func vpp_acl_add_replace_rule(vppRule *acl.ACLRule) error {
	conn := govpp.Connect()
	defer conn.Disconnect()

	ch := conn.NewApiChannel()
	defer ch.Close()

	req := &acl.ACLAddReplace{
		ACLIndex: ^uint32(0),
		Tag:      []byte(vppRule.RuleId),
		R: []acl.ACLRule{
			{
				IsPermit:       vppRule.IsPermit,
				SrcIPAddr:      vppRule.SrcIPAddr,
				SrcIPPrefixLen: vppRule.SrcIPPrefixLen,
				DstIPAddr:      vppRule.DstIPAddr,
				DstIPPrefixLen: vppRule.DstIPPrefixLen,
				Proto:          vppRule.Proto,
				SrcportOrIcmptypeFirst: vppRule.SrcportOrIcmptypeFirst,
				SrcportOrIcmptypeLast:  vppRule.SrcportOrIcmptypeLast,
				DstportOrIcmpcodeFirst: vppRule.DstportOrIcmpcodeFirst,
				DstportOrIcmpcodeLast:  vppRule.DstportOrIcmpcodeLast,
			},
		},
	}

	// send the request - channel API instead of SendRequest
	ch.ReqChan <- &api.VppRequest{Message: req}

	// receive the response - channel API instead of ReceiveReply
	vppReply := <-ch.ReplyChan
	reply := &acl.ACLAddReplaceReply{}
	ch.Decoder.DecodeMsg(vppReply.Data, reply)

	fmt.Printf("%+v\n", reply)
	if reply.Retval != 0 {
		return errors.New("Could not add set af_packet interface flag, admin state up")
	}
	vppIndexValue := vppRuleT{
		reply.ACLIndex,
	}
	vppRuleByID[vppRule.RuleId] = &vppIndexValue
	return nil
}

func vpp_acl_del_rule(vppRule *acl.ACLRule) error {
	conn := govpp.Connect()
	defer conn.Disconnect()

	ch := conn.NewApiChannel()
	defer ch.Close()

	req := &acl.ACLDel{
		ACLIndex: vppRuleByID[vppRule.RuleId].index,
	}

	// send the request - channel API instead of SendRequest
	ch.ReqChan <- &api.VppRequest{Message: req}

	// receive the response - channel API instead of ReceiveReply
	vppReply := <-ch.ReplyChan
	reply := &acl.ACLDelReply{}
	ch.Decoder.DecodeMsg(vppReply.Data, reply)

	fmt.Printf("%+v\n", reply)
	if reply.Retval != 0 {
		return errors.New("Could not add set af_packet interface flag, admin state up")
	}
	return nil
}
