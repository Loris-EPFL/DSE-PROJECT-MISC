package impl

import (
	"time"

	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

func (n *node) runHeartBeatMechanism() {

	defer n.wg.Done()
	log := n.getLogger()

	firstBeat := n.sendHeartBeat()
	if firstBeat != nil {
		log.Error().
			Msgf("Unable to send first heartbeat from %s", n.conf.Socket.GetAddress())
	}

	defer log.Warn().Msg("Finished runHeartBeatMechanism")
	for {
		select {
		case <-n.stopCh:

			return
		case <-time.After(n.conf.HeartbeatInterval):
			err := n.sendHeartBeat()
			if err != nil {
				log.Error().Err(err).Msg("Failed to send heartbeat")
			}
		}

	}

}

// function that runs the anti entropy mechanism with a ticker using the AntiEntropyInterval
func (n *node) runStatusMechanism() {
	log := n.getLogger()
	defer n.wg.Done()

	defer log.Warn().Msg("Finished runStatusMechanism")

	for {
		select {
		case <-n.stopCh:
			return
		case <-time.After(n.conf.AntiEntropyInterval):
			err := n.sendRandomStatusMessage()
			if err != nil {
				log.Error().
					Msgf("Impossible to send random status message from %s",
						n.conf.Socket.GetAddress())
			}
		}
	}
}

func (n *node) broadCastRoutine(rumor *types.RumorsMessage) {
	log := n.getLogger()

	neighbors := n.getNeighbors()

	log.Info().Msgf("Neighbors: %v", neighbors)

	randomNeighbor := getRandom(neighbors)

	if len(randomNeighbor) == 0 {
		log.Warn().
			Msgf("No neighbors to broadcast from %s", n.conf.Socket.GetAddress())
		return
	}

	transportMsg, err := n.conf.MessageRegistry.MarshalMessage(rumor)
	if err != nil {
		log.Error().
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
		log.Error().
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

	log := n.getLogger()
	log.Info().Msg("Node started.")
	for {
		select {
		case <-n.stopCh:
			log.Info().Msg("Stopping node.")
			return
		default:
			if err := n.handleIncomingPacket(); err != nil {
				log.Error().Msgf("%v", err)
			}
		}
	}
}
