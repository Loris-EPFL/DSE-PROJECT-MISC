package impl

import (
	"sync"
	"time"

	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/types"
)

// Begin of DNS struct by Loris
// DNSEntry represents a DNS entry with a domain name, IP address, and expiration date.
type DNSEntry struct {
	Domain     string
	IPAddress  string
	Expiration time.Time
}

type sequenceNumber struct {
	mu        sync.Mutex
	seqNumber uint
}

type node struct {
	peer.Peer
	conf           peer.Configuration
	wg             sync.WaitGroup
	sequenceNumber sequenceNumber

	stopCh                chan struct{}
	ackChannel            SafeMap[string, chan *types.AckMessage]
	directPeers           SafeMap[string, struct{}]
	receivedSeq           SafeMap[string, uint]
	routingTable          SafeMap[string, string]
	rumorDB               SafeMap[string, types.RumorByOrigin]
	namingStorage         SafeMap[string, string]
	dataReplyChan         SafeMap[string, chan *types.DataReplyMessage]
	searchReplyChan       SafeMap[string, chan *types.SearchReplyMessage]
	handledDataRequests   SafeMap[string, string]
	handledSearchRequests SafeMap[string, string]
	catalog               SafeCatalog

	//Added DNS store
	dnsStore SafeMap[string, DNSEntry]
}

func NewPeer(conf peer.Configuration) peer.Peer {
	n := &node{
		conf:                  conf,
		sequenceNumber:        sequenceNumber{seqNumber: 0},
		directPeers:           NewSafeMap[string, struct{}](),
		routingTable:          NewSafeMap[string, string](),
		receivedSeq:           NewSafeMap[string, uint](),
		rumorDB:               NewSafeMap[string, types.RumorByOrigin](),
		namingStorage:         NewSafeMap[string, string](),
		handledDataRequests:   NewSafeMap[string, string](),
		handledSearchRequests: NewSafeMap[string, string](),
		stopCh:                make(chan struct{}),
		ackChannel:            NewSafeMap[string, chan *types.AckMessage](),
		dataReplyChan:         NewSafeMap[string, chan *types.DataReplyMessage](),
		searchReplyChan:       NewSafeMap[string, chan *types.SearchReplyMessage](),
		catalog:               NewSafeCatalog(),
		dnsStore:              NewSafeMap[string, DNSEntry](),
	}

	//initialize auxiliary structures
	n.routingTable.Add(conf.Socket.GetAddress(), conf.Socket.GetAddress())

	// register the callback for chat messages
	n.conf.MessageRegistry.RegisterMessageCallback(types.ChatMessage{}, n.handleChatMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.RumorsMessage{}, n.handleRumor)
	n.conf.MessageRegistry.RegisterMessageCallback(types.StatusMessage{}, n.handleStatusMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.AckMessage{}, n.handleAckMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.EmptyMessage{}, n.handleEmptyMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.PrivateMessage{}, n.handlePrivateMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.DataRequestMessage{}, n.handleDataRequestMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.DataReplyMessage{}, n.handleDataReplyMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.SearchRequestMessage{}, n.handleSearchRequestMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.SearchReplyMessage{}, n.handleSearchReplyMessage)

	//Added DNS messages handlers
	// Register the callback for DNS messages
	n.conf.MessageRegistry.RegisterMessageCallback(types.DNSReadMessage{}, n.handleDNSReadMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.DNSRenewalMessage{}, n.handleDNSRenewalMessage)
	n.conf.MessageRegistry.RegisterMessageCallback(types.DNSRegisterMessage{}, n.handleDNSRegisterMessage)

	return n
}

// Start implements peer.Service
func (n *node) Start() error {

	log := n.getLogger()
	address := n.conf.Socket.GetAddress()
	log.Info().Msgf("Node %s starting to listen to packets", address)
	n.wg.Add(1)
	go n.listen()

	if n.conf.AntiEntropyInterval > 0 {
		log.Info().Msgf("Node %s starting anti entropy mechanism", address)
		n.wg.Add(1)
		go n.runStatusMechanism()
	}

	if n.conf.HeartbeatInterval > 0 {
		log.Info().Msgf("Node %s sending first heartbeat", address)
		n.wg.Add(1)
		go n.runHeartBeatMechanism()
	}

	return nil
}

// Stop implements peer.Service
func (n *node) Stop() error {
	log := n.getLogger()
	close(n.stopCh)
	log.Info().Msg("WAITING FOR ROUTINE TO FINISH")
	n.wg.Wait()
	log.Info().Msg("ALL ROUTINES FINISHED")
	return nil
}
