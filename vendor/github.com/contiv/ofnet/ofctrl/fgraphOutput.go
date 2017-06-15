/***
Copyright 2014 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package ofctrl

// This file implements the forwarding graph API for the output element

import (
	"github.com/contiv/libOpenflow/openflow13"
)

type Output struct {
	outputType string // Output type: "drop", "toController" or "port"
	portNo     uint32 // Output port number
}

// Fgraph element type for the output
func (self *Output) Type() string {
	return "output"
}

// instruction set for output element
func (self *Output) GetFlowInstr() openflow13.Instruction {
	outputInstr := openflow13.NewInstrApplyActions()

	switch self.outputType {
	case "drop":
		return nil
	case "toController":
		outputAct := openflow13.NewActionOutput(openflow13.P_CONTROLLER)
		// Dont buffer the packets being sent to controller
		outputAct.MaxLen = openflow13.OFPCML_NO_BUFFER
		outputInstr.AddAction(outputAct, false)
	case "normal":
		fallthrough
	case "port":
		outputAct := openflow13.NewActionOutput(self.portNo)
		outputInstr.AddAction(outputAct, false)
	}

	return outputInstr
}

// Return an output action (Used by group mods)
func (self *Output) GetOutAction() openflow13.Action {
	switch self.outputType {
	case "drop":
		return nil
	case "toController":
		outputAct := openflow13.NewActionOutput(openflow13.P_CONTROLLER)
		// Dont buffer the packets being sent to controller
		outputAct.MaxLen = openflow13.OFPCML_NO_BUFFER

		return outputAct
	case "normal":
		fallthrough
	case "port":
		return openflow13.NewActionOutput(self.portNo)
	}

	return nil
}
