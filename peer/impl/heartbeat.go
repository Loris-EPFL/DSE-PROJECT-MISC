package impl

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// Global logger function
func init() {
	// defaultLevel can be changed to set the desired level of the logger
	defaultLevel = zerolog.InfoLevel

	if os.Getenv("GLOG") == "no" {
		defaultLevel = zerolog.Disabled
	}

	logger = zerolog.New(logout).
		Level(defaultLevel).
		With().Timestamp().Logger().
		With().Caller().Logger().
		With().Str("role", "heartbeat.go").Logger()
}

// function that picks a random neighbor and sends it a status message
func (n *node) sendRandomStatusMessage() error {

	//Get random neighbor
	neighbors := n.getNeighbors()

	if len(neighbors) == 0 {
		return xerrors.Errorf("No friends to gossip with")
	}
	randNeighbor := getRandom(neighbors)

	//create the statusMessage which is a view
	statusMsg := n.createStatusMessage()

	//Marshalling the message
	transpStatusMsg, err := n.conf.MessageRegistry.MarshalMessage(&statusMsg)
	if err != nil {
		return xerrors.Errorf("Status message unmarshable, got %T", err)
	}
	//create the packet
	statusHeader := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		randNeighbor,
	)
	statusPkt := createTransportPacket(&statusHeader, &transpStatusMsg)

	//send the status packet
	err = n.conf.Socket.Send(randNeighbor, statusPkt, time.Second*1)
	if err != nil {
		return xerrors.Errorf("Status message ungossipable, got %T", err)
	}

	logger.Info().Msgf("Sent StatusMessage to %s", randNeighbor)

	return nil
}

func (n *node) sendHeartBeat() error {

	emptyMsg := types.EmptyMessage{}

	emptyMsgTransp, err := n.conf.MessageRegistry.MarshalMessage(&emptyMsg)

	if err != nil {
		logger.Error().Err(err).Msg("failed to marshal empty message")
		return err
	}

	transportMsg := transport.Message{
		Type:    emptyMsgTransp.Type,
		Payload: emptyMsgTransp.Payload,
	}

	err = n.Broadcast(transportMsg)

	if err != nil {
		logger.Error().Err(err).Msg("Failed to broadcast heartbeat")
		return err
	}

	logger.Info().Msg("Heartbeat sent")
	return nil

}
