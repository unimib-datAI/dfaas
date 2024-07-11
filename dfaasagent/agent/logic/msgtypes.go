package logic

import (
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
)

//////////////////// MESSAGES' STRUCT TYPES ////////////////////

// MsgText defines the format of the PubSub messages containing a bare text message
type MsgText struct {
	MsgType string

	Text string
}

// StrMsgTextType value for MsgText.MsgType
const StrMsgTextType = "text"

// MsgNodeInfoRecalc defines the format of the PubSub messages regarding a node's
// information (for Recalc strategy)
type MsgNodeInfoRecalc struct {
	MsgType string

	HAProxyHost string
	HAProxyPort uint

	// FuncLimits is a nested structure consisting of two maps. The mapping is
	// the following: the rate limit for function funcName on node nodeID can be
	// obtained by writing FuncLimits[nodeID][funcName]
	FuncLimits map[string]map[string]float64
}

// StrMsgNodeInfoTypeRecalc value for MsgNodeInfo.MsgType
const StrMsgNodeInfoTypeRecalc = "nodeinfoRecalc"

// MsgNodeInfoNMS defines the format of the PubSub messages regarding a node's
// information (for Node Margin Strategy)
type MsgNodeInfoNMS struct {
	MsgType string

	HAProxyHost string
	HAProxyPort uint
	NodeType 	int
	Functions 	[]string
}

// StrMsgNodeInfoTypeNMS value for MsgNodeInfoNMS.MsgType
const StrMsgNodeInfoTypeNMS = "nodeinfoNMS"

// MsgNodeMarginInfoNMS defines the format of the PubSub messages regarding a node's
// margin and eventually the expected load (for Node Margin Strategy)
type MsgNodeMarginInfoNMS struct {
	MsgType string

	Margin 		float64
	Load 		groupsLoad
}

// StrMsgNodeMarginInfoTypeNMS value for MsgNodeMarginInfoNMS.MsgType
const StrMsgNodeMarginInfoTypeNMS = "nodemargininfoNMS"

// processMsgText processes a text message received from pubsub
func processMsgText(sender string, msg *MsgText) error {
	logger := logging.Logger()
	myself := _p2pHost.ID().String()

	if sender == myself {
		return nil // Ignore ourselves
	}

	logger.Debugf("Received text message from node %s: %s", sender, msg.Text)

	return nil
}
