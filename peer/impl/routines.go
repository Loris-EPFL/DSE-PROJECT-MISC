package impl

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
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
		With().Str("role", "routines.go").Logger()
}

func (n *node) runHeartBeatMechanism() {

	defer n.wg.Done()

	firstBeat := n.sendHeartBeat()
	if firstBeat != nil {
		logger.Error().
			Msgf("Unable to send first heartbeat from %s", n.conf.Socket.GetAddress())
	}

	defer logger.Warn().Msg("Finished runHeartBeatMechanism")
	for {
		select {
		case <-n.stopCh:

			return
		case <-time.After(n.conf.HeartbeatInterval):
			err := n.sendHeartBeat()
			if err != nil {
				logger.Error().Err(err).Msg("Failed to send heartbeat")
			}
		}

	}

}

// function that runs the anti entropy mechanism with a ticker using the AntiEntropyInterval
func (n *node) runStatusMechanism() {

	defer n.wg.Done()

	defer logger.Warn().Msg("Finished runStatusMechanism")

	for {
		select {
		case <-n.stopCh:
			return
		case <-time.After(n.conf.AntiEntropyInterval):
			err := n.sendRandomStatusMessage()
			if err != nil {
				logger.Error().
					Msgf("Impossible to send random status message from %s",
						n.conf.Socket.GetAddress())
			}
		}
	}
}

func (n *node) broadCastRoutine(rumor *types.RumorsMessage) {

	neighbors := n.getNeighbors()

	logger.Info().Msgf("Neighbors: %v", neighbors)

	randomNeighbor := getRandom(neighbors)

	if len(randomNeighbor) == 0 {
		logger.Warn().
			Msgf("No neighbors to broadcast from %s", n.conf.Socket.GetAddress())
		return
	}

	transportMsg, err := n.conf.MessageRegistry.MarshalMessage(rumor)
	if err != nil {
		logger.Error().
			Msgf("failed to marshal RumorsMessage: %v", err)
		return
	}

	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		randomNeighbor,
	)

	pkt := createTransportPacket(&header, &transportMsg)

	if err := n.conf.Socket.Send(randomNeighbor, pkt, time.Second*1); err != nil {
		logger.Error().
			Str("destination", randomNeighbor).
			Err(err).
			Msg("Failed to send rumor message")
		return
	}

	//We can wait for ack here

	go n.waitForAck(pkt, randomNeighbor)

}

func (n *node) listen() {

	defer n.wg.Done()

	logger.Info().Msg("Node started.")
	for {
		select {
		case <-n.stopCh:
			logger.Info().Msg("Stopping node.")
			return
		default:
			if err := n.handleIncomingPacket(); err != nil {
				logger.Error().Msgf("%v", err)
			}
		}
	}
}
