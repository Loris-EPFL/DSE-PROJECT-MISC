// communication.go

package impl

import (
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// auxiliary function that returns the next hop in the routing table of a node for a certain destination
func (n *node) getNextHop(destination string) (string, error) {
	log := n.getLogger()

	nextHop, exists := n.routingTable.Get(destination)
	if !exists {
		log.Error().
			Str("destination", destination).
			Msg("No route to destination")
		return "", xerrors.Errorf("No route to destination %s", destination)
	}
	return nextHop, nil
}

// GetRoutingTable implements peer.Messaging
func (n *node) GetRoutingTable() peer.RoutingTable {

	cpy := n.routingTable.ToMap()
	for k, v := range cpy {
		cpy[k] = v
	}
	return cpy
}

// SetRoutingEntry implements peer.Messaging
func (n *node) SetRoutingEntry(origin, relayAddr string) {

	if relayAddr == "" {
		n.routingTable.Remove(origin)
	} else {
		n.routingTable.Add(origin, relayAddr)
	}
}

// AddPeer implements peer.Messaging
func (n *node) AddPeer(addr ...string) {

	// Adding ourselves should have no effect
	for _, a := range addr {
		if a != n.conf.Socket.GetAddress() {
			n.directPeers.Add(a, struct{}{})
			n.routingTable.Add(a, a)
		}
	}
}

// Unicast implements peer.Messaging
func (n *node) Unicast(dest string, msg transport.Message) error {
	log := n.getLogger()

	nextHop, ok := n.routingTable.Get(dest)

	if !ok {
		log.Error().
			Str("destination", dest).
			Msg("No route to destination")
		return xerrors.Errorf("No route to destination %s", dest)
	}

	// Create the header
	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		dest,
	)

	// Create the package
	pkt := createTransportPacket(&header, &msg)

	// send the message
	if err := n.conf.Socket.Send(nextHop, pkt, time.Second*1); err != nil {
		log.Error().
			Str("destination", dest).
			Str("next_hop", nextHop).
			Err(err).
			Msg("Failed to send unicast message")
		return err
	}

	log.Info().
		Str("destination", dest).
		Str("next_hop", nextHop).
		Msg("Unicast message sent")

	return nil
}

// *Notes:
// * - Create a RumorsMessage containing one Rumor (that embeds the msg provided
// * - in argument) and sends it to a random neighbor
// * - Process the message locally by creating a packet and calling processpacket
func (n *node) Broadcast(msg transport.Message) error {

	//increment seq number

	currentSequenceNumber := n.sequenceNumber.getSequenceNumber()

	n.sequenceNumber.updateSeqNumber(currentSequenceNumber + 1)

	n.receivedSeq.Add(n.conf.Socket.GetAddress(), currentSequenceNumber+1)

	//create a rumor of the message to broadcast
	rumor := types.Rumor{
		Origin:   n.conf.Socket.GetAddress(),
		Sequence: n.sequenceNumber.seqNumber,
		Msg:      &msg,
	}

	//embed the rumor in a rumor structure
	rumorsMessage := types.RumorsMessage{
		Rumors: []types.Rumor{rumor},
	}

	n.storeRumor(rumor)
	go n.broadCastRoutine(&rumorsMessage)

	// // Processing the message locally

	// // Creating the local header
	// localHeader := transport.NewHeader(
	//     n.conf.Socket.GetAddress(),
	//     n.conf.Socket.GetAddress(),
	//     n.conf.Socket.GetAddress(),
	// )

	// //.. and the associated packet
	// pkt := createTransportPacket(&localHeader, rumor.Msg)

	// process := n.conf.MessageRegistry.ProcessPacket(pkt)

	// if process != nil {

	//     return process
	// }

	return nil
}
