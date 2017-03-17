// Package uflow provides the Go interface to VPP binary API of the uflow VPP module.
// Generated from 'uflow.api.json' on Fri, 17 Mar 2017 17:11:50 UTC.
package uflow

// VlApiVersion contains version of the API.
const VlAPIVersion = 0x85909300

// UflowIdx is the Go representation of the VPP binary API data type 'uflow_idx'.
type UflowIdx struct {
	Vslot uint32
	Md    uint32
	Sid   uint32
}

func (*UflowIdx) GetTypeName() string {
	return "uflow_idx"
}
func (*UflowIdx) GetCrcString() string {
	return "3310d92c"
}

// UflowEnt is the Go representation of the VPP binary API data type 'uflow_ent'.
type UflowEnt struct {
	CmDpidx      uint32
	VbundleDpidx uint32
}

func (*UflowEnt) GetTypeName() string {
	return "uflow_ent"
}
func (*UflowEnt) GetCrcString() string {
	return "50fa3f43"
}

// UflowRow is the Go representation of the VPP binary API data type 'uflow_row'.
type UflowRow struct {
	Idx UflowIdx
	Ent UflowEnt
}

func (*UflowRow) GetTypeName() string {
	return "uflow_row"
}
func (*UflowRow) GetCrcString() string {
	return "3b73b975"
}

// UflowEnableDisable is the Go representation of the VPP binary API message 'uflow_enable_disable'.
type UflowEnableDisable struct {
	SwIfIndex     uint32
	EnableDisable uint8
}

func (*UflowEnableDisable) GetMessageName() string {
	return "uflow_enable_disable"
}
func (*UflowEnableDisable) GetCrcString() string {
	return "4c7f1b8a"
}

// UflowEnableDisableReply is the Go representation of the VPP binary API message 'uflow_enable_disable_reply'.
type UflowEnableDisableReply struct {
	Retval int32
}

func (*UflowEnableDisableReply) GetMessageName() string {
	return "uflow_enable_disable_reply"
}
func (*UflowEnableDisableReply) GetCrcString() string {
	return "f47b6600"
}

// UflowSetEnt is the Go representation of the VPP binary API message 'uflow_set_ent'.
type UflowSetEnt struct {
	Idx UflowIdx
	Ent UflowEnt
}

func (*UflowSetEnt) GetMessageName() string {
	return "uflow_set_ent"
}
func (*UflowSetEnt) GetCrcString() string {
	return "6bfeac11"
}

// UflowSetEntReply is the Go representation of the VPP binary API message 'uflow_set_ent_reply'.
type UflowSetEntReply struct {
	Retval int32
}

func (*UflowSetEntReply) GetMessageName() string {
	return "uflow_set_ent_reply"
}
func (*UflowSetEntReply) GetCrcString() string {
	return "c49943f5"
}

// UflowClrEnt is the Go representation of the VPP binary API message 'uflow_clr_ent'.
type UflowClrEnt struct {
	Idx UflowIdx
}

func (*UflowClrEnt) GetMessageName() string {
	return "uflow_clr_ent"
}
func (*UflowClrEnt) GetCrcString() string {
	return "9c0b61a7"
}

// UflowClrEntReply is the Go representation of the VPP binary API message 'uflow_clr_ent_reply'.
type UflowClrEntReply struct {
	Retval int32
}

func (*UflowClrEntReply) GetMessageName() string {
	return "uflow_clr_ent_reply"
}
func (*UflowClrEntReply) GetCrcString() string {
	return "6ca429f7"
}

// UflowDump is the Go representation of the VPP binary API message 'uflow_dump'.
type UflowDump struct {
}

func (*UflowDump) GetMessageName() string {
	return "uflow_dump"
}
func (*UflowDump) GetCrcString() string {
	return "f0ac7601"
}

// UflowDumpReply is the Go representation of the VPP binary API message 'uflow_dump_reply'.
type UflowDumpReply struct {
	Retval int32
	Num    uint32 `struc:"sizeof=Row"`
	Row    []UflowRow
}

func (*UflowDumpReply) GetMessageName() string {
	return "uflow_dump_reply"
}
func (*UflowDumpReply) GetCrcString() string {
	return "85b96451"
}
