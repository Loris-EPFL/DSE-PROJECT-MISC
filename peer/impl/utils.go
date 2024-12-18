package impl

import (
	"math/rand"
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.dedis.ch/cs438/transport"
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
		With().Str("role", "utils.go").Logger()
}

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

	logger.Info().
		Str("from", pkt.Header.Source).
		Str("to", pkt.Header.Destination).
		Msg("Relaying packet")

	// Create new header
	newHeader := transport.NewHeader(pkt.Header.Source, n.conf.Socket.GetAddress(), pkt.Header.Destination)
	pkt.Header = &newHeader

	// Get next hop in the routing table
	nextHop, err := n.getNextHop(pkt.Header.Destination)
	if err != nil {
		logger.Error().
			Str("destination", pkt.Header.Destination).
			Err(err).
			Msg("Failed to get next hop")
		return xerrors.Errorf("Failed to get next hop %v", err)
	}

	logger.Info().
		Str("destination", pkt.Header.Destination).
		Str("next_hop", nextHop).
		Msg("Sending packet to next hop")

	if err := n.conf.Socket.Send(nextHop, pkt, time.Second); err != nil {
		logger.Error().
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
