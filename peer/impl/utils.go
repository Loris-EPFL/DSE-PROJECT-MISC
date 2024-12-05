package impl

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog"
	"go.dedis.ch/cs438/transport"
	"golang.org/x/xerrors"
)

// for verbose purposes
func createTransportPacket(header *transport.Header, message *transport.Message) transport.Packet {
	return transport.Packet{
		Header: header,
		Msg:    message,
	}
}

// Gets a random element from a slice
// For verbose purposes
func getRandom(slice []string) string {

	if len(slice) == 0 {
		return ""
	}
	return slice[rand.Intn(len(slice))]
}

// Helper method to get the node-specific logger
func (n *node) getLogger() zerolog.Logger {
	return logger.With().Str("address", n.conf.Socket.GetAddress()).Logger()
}

// function that processes packet in case the packet is for the actual node
func (n *node) processPacket(pkt transport.Packet) error {
	return n.conf.MessageRegistry.ProcessPacket(pkt)
}

// auxiliary function that relays the packet to another node
func (n *node) relayPacket(pkt transport.Packet) error {
	log := n.getLogger()
	log.Info().
		Str("from", pkt.Header.Source).
		Str("to", pkt.Header.Destination).
		Msg("Relaying packet")

	// Create new header
	newHeader := transport.NewHeader(pkt.Header.Source, n.conf.Socket.GetAddress(), pkt.Header.Destination)
	pkt.Header = &newHeader

	// Get next hop in the routing table
	nextHop, err := n.getNextHop(pkt.Header.Destination)
	if err != nil {
		log.Error().
			Str("destination", pkt.Header.Destination).
			Err(err).
			Msg("Failed to get next hop")
		return xerrors.Errorf("Failed to get next hop %v", err)
	}

	log.Info().
		Str("destination", pkt.Header.Destination).
		Str("next_hop", nextHop).
		Msg("Sending packet to next hop")

	if err := n.conf.Socket.Send(nextHop, pkt, time.Second); err != nil {
		log.Error().
			Str("destination", pkt.Header.Destination).
			Str("next_hop", nextHop).
			Err(err).
			Msg("Failed to send packet")
		return xerrors.Errorf("Failed to send packet to %s via %s, %v", pkt.Header.Destination, nextHop, err)
	}
	return nil
}

func (s *sequenceNumber) updateSeqNumber(seq uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seqNumber = seq
}

func (s *sequenceNumber) getSequenceNumber() uint {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seqNumber
}

// GetDNSStore returns the DNS store
func (n *node) GetDNSStore() SafeMap[string, DNSEntry] {
	return n.dnsStore
}

/*
// GetDNSStoreCopy returns a deep copy of the DNS store
func (n *node) GetDNSStoreCopy() SafeMap[string, DNSEntry] {
	originalStore := n.GetDNSStore()
	copyStore := SafeMap[string, DNSEntry]{
		store: make(map[string]DNSEntry),
	}

	originalStore.mu.RLock()
	defer originalStore.mu.RUnlock()

	for key, value := range originalStore.store {
		copyStore.store[key] = value
	}

	return copyStore
}
*/
