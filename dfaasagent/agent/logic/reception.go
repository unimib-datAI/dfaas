package logic

import (
	"encoding/json"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"gitlab.com/team-dfaas/dfaas/node-stack/dfaasagent/agent/logging"
)

//////////////////// PUBLIC FUNCTIONS FOR RECEPTION ////////////////////

// OnReceived should be executed every time a message from a peer is received
func OnReceived(msg *pubsub.Message) error {
	var msgForType struct{ MsgType string }
	err := json.Unmarshal(msg.GetData(), &msgForType)
	if err != nil {
		return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
	}

	switch msgForType.MsgType {
	case StrMsgTextType:
		var objMsg MsgText
		err := json.Unmarshal(msg.GetData(), &objMsg)
		if err != nil {
			return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
		}

		processMsgText(msg.GetFrom().String(), &objMsg)
	case StrMsgNodeInfoType:
		var objMsg MsgNodeInfo
		err := json.Unmarshal(msg.GetData(), &objMsg)
		if err != nil {
			return errors.Wrap(err, "Error while deserializing a message from the PubSub subscription")
		}

		processMsgNodeInfo(msg.GetFrom().String(), &objMsg)
	}

	return nil
}

//////////////////// PRIVATE FUNCTIONS FOR RECEPTION ////////////////////

// processMsgText processes a text message received from pubsub
func processMsgText(sender string, msg *MsgText) error {
	logger := logging.Logger()
	myself := _p2pHost.ID().String()

	if sender == myself {
		return nil // Ignore ourselves
	}

	logger.Info("Received text message from node \"", sender, "\": ", msg.Text)

	return nil
}

// processMsgNodeInfo processes a node info message received from pubsub
func processMsgNodeInfo(sender string, msg *MsgNodeInfo) error {
	myself := _p2pHost.ID().String()

	if sender == myself {
		return nil // Ignore ourselves
	}

	// Note: if the sender node do not "know" us (we aren't in his FuncLimits) we just ignore his message
	funcLimits, present := msg.FuncLimits[myself]
	if present {
		_nodestbl.SetReceivedValues(sender, msg.HAProxyHost, msg.HAProxyPort, funcLimits)
	}

	return nil
}
