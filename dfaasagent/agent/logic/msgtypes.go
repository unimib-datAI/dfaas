package logic

//////////////////// MESSAGES' STRUCT TYPES ////////////////////

// MsgText defines the format of the PubSub messages containing a bare text message
type MsgText struct {
	MsgType string

	Text string
}

// StrMsgTextType value for MsgText.MsgType
const StrMsgTextType = "text"

// MsgNodeInfo defines the format of the PubSub messages regarding a node's
// information
type MsgNodeInfo struct {
	MsgType string

	HAProxyHost string
	HAProxyPort uint

	// FuncLimits is a nested structure consisting of two maps. The mapping is
	// the following: the rate limit for function funcName on node nodeID can be
	// obtained by writing FuncLimits[nodeID][funcName]
	FuncLimits map[string]map[string]float64
}

// StrMsgNodeInfoType value for MsgNodeInfo.MsgType
const StrMsgNodeInfoType = "nodeinfo"
