package impl

import (
	"math/rand"
	"regexp"
	"time"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// auxiliary function that handles incoming packets
func (n *node) handleIncomingPacket() error {
	log := n.getLogger()
	pkt, err := n.conf.Socket.Recv(time.Second)
	if err != nil {
		return err
	}

	log.Info().
		Str("from", pkt.Header.RelayedBy).
		Str("to", pkt.Header.Destination).
		Str("type", pkt.Msg.Type).
		Str("packetID", pkt.Header.PacketID).
		Msg("Received packet")

	if pkt.Header.Destination == n.conf.Socket.GetAddress() {
		return n.processPacket(pkt)
	}

	return n.relayPacket(pkt)
}

func (n *node) handlePrivateMessage(msg types.Message, pkt transport.Packet) error {
	log := n.getLogger()

	privMsg, ok := msg.(*types.PrivateMessage)

	if !ok {
		log.Error().
			Msgf("Expected PrivateMessage, got %T", msg)
		return xerrors.Errorf("expected PrivateMessage, got %T", msg)
	}

	isRecip := false

	for recipient := range privMsg.Recipients {
		if recipient == n.conf.Socket.GetAddress() {
			isRecip = true
			break
		}
	}

	if isRecip {
		log.Info().Msg("Processing PrivateMessage")

		privPkt := createTransportPacket(pkt.Header, privMsg.Msg)

		err := n.conf.MessageRegistry.ProcessPacket(privPkt)

		if err != nil {
			log.Error().Err(err).Msg("Failed to process embedded message")
			return xerrors.Errorf("Failed to process embedded message")
		}
	} else {
		log.Info().Msg("Not a recipient of PrivateMessage; ignoring")

	}

	return nil
}

func (n *node) handleEmptyMessage(msg types.Message, pkt transport.Packet) error {
	return nil
}

func (n *node) handleChatMessage(msg types.Message, pkt transport.Packet) error {
	log := n.getLogger()
	chatMsg, ok := msg.(*types.ChatMessage)

	if !ok {
		log.Error().
			Msgf("Expected ChatMessage, got %T", msg)
		return xerrors.Errorf("expected ChatMessage, got %T", msg)
	}

	log.Info().
		Str("message", chatMsg.Message).
		Msg("Received chat message")

	return nil
}

func (n *node) handleStatusMessage(msg types.Message, pkt transport.Packet) error {
	log := n.getLogger()
	statusMsg, ok := msg.(*types.StatusMessage)

	if !ok {
		log.Error().
			Msgf("Expected status message, got %T", msg)
		return xerrors.Errorf("Expected status message, got %T", msg)
	}

	c, rumorsToSend := n.compareViews(*statusMsg)
	//have to compare n view with received "remote" view

	switch c {
	//1. Remote has views that local hasn't
	case 1:
		err := n.sendStatusMessage(pkt.Header.Source)
		if err != nil {
			log.Error().Msgf("Impossible to send status message to %s", pkt.Header.Source)
		}

	//2. local has views that remote hasn't
	case 2:
		err := n.sendMissingRumors(pkt.Header.Source, rumorsToSend)
		if err != nil {
			log.Error().Msgf("Impossible to send missing rumors to to %s", pkt.Header.Source)
		}

	//3. Both have views that the other hasn't
	case 3:
		errStat := n.sendStatusMessage(pkt.Header.Source)

		errMiss := n.sendMissingRumors(pkt.Header.Source, rumorsToSend)

		if errStat != nil || errMiss != nil {
			log.Error().
				Msgf("Unable to send status or missing rumors to %s", pkt.Header.Source)

		}
	//4. Same view
	case 4:
		n.continueMongering(pkt.Header.Source)
	}

	return nil
}

func (n *node) handleAckMessage(msg types.Message, pkt transport.Packet) error {

	log := n.getLogger()

	ackMsg, ok := msg.(*types.AckMessage)

	if !ok {
		log.Error().
			Msgf("Expected AckMessage, got %T", msg)
		return xerrors.Errorf("Expected AckMessage got %T", msg)
	}

	goodChan, exists := n.ackChannel.Get(ackMsg.AckedPacketID)

	if !exists {
		log.Error().Msgf("No acked packet ID found")
	} else {
		goodChan <- ackMsg
	}

	log.Warn().Msgf("Channel %v", goodChan)

	statusMsg := ackMsg.Status

	statusMsgTransp, err := n.conf.MessageRegistry.MarshalMessage(statusMsg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal StatusMessage")
		return err
	}

	statusPkt := transport.Packet{
		Header: pkt.Header,
		Msg:    &statusMsgTransp,
	}

	//process the status message
	return n.conf.MessageRegistry.ProcessPacket(statusPkt)
}

func (n *node) handleRumor(msg types.Message, pkt transport.Packet) error {

	log := n.getLogger()
	rumors, ok := msg.(*types.RumorsMessage)

	if !ok {
		log.Error().Msgf("Expected rumor, got %T", msg)
		return nil
	}

	processedRumor := false

	//rumor is a types.RumorsMessage that can contain one or multiple rumors
	//have to iterate over all rumors and broadcast them

	for _, rumor := range rumors.Rumors {

		og := rumor.Origin    //we have to check the sequence number to see whether the rumor is expected
		seq := rumor.Sequence //should correspond to the sequence number of the origin seq number

		lastSeq, exists := n.receivedSeq.Get(og)
		//lastSeq corresponds to the sequence number of the last rumor sent by origin to this node
		if !exists {
			lastSeq = 0
		}

		expectedSequence := lastSeq + 1 //prepare for next rumor
		if seq == expectedSequence {

			err := n.processRumorMessage(rumor, pkt)
			if err != nil {
				log.Error().
					Msg("Failed to process Rumor message")

				return err
			}

			n.receivedSeq.Add(og, seq) //update sequence number of last sent rumor by origin
			processedRumor = true

			//Update routing table of node n thanks to rumors

			_, isNeighbor := n.directPeers.Get(rumor.Origin)

			if !isNeighbor && rumor.Origin != n.conf.Socket.GetAddress() {
				n.SetRoutingEntry(rumor.Origin, pkt.Header.RelayedBy)

			}

			ackk := n.sendAckMessage(pkt)

			if ackk != nil {
				log.Error().
					Msgf("Failed to send Ack from %s to %s: %v.",
						n.conf.Socket.GetAddress(),
						pkt.Header.Source, ackk)

				return ackk
			}
		} else {
			continue
		}
	}

	//gossip baby
	if processedRumor {
		err := n.forwardRumorsMessage(rumors, pkt)
		if err != nil {
			log.Error().
				Msgf("Unable to forward rumor message from %s", n.conf.Socket.GetAddress())
		}

	}
	return nil
}

/**
 * Node 1 adds 2 and 3
 * Node 1 broadcasts to 2
 * 2 acks 1
 * 1 receives ack and process status: same view -> Continue Mongering
 * 1 sends status to 3
 * 3 receives status and compare with own status: -> remote node has smth that
 * I don't have -> Send him status message
 * 1 receives 3 status message and process it: i have something other node hasn't

 * 1 sends missing rumor to 3

 * 3 receives and sends ack
 * 1 receives ack and done
 */

// * Data and search message handlers
func (n *node) handleDataRequestMessage(msg types.Message, pkt transport.Packet) error {

	log := n.getLogger()

	dataRequest, ok := msg.(*types.DataRequestMessage)

	if n.handledDataRequests.Exists(dataRequest.RequestID) {
		return nil
	}

	if !ok {
		return xerrors.Errorf("Expected DataRequestMessage got %T", msg)
	}

	blobStore := n.conf.Storage.GetDataBlobStore()
	value := blobStore.Get(dataRequest.Key)

	dataReply := &types.DataReplyMessage{
		RequestID: dataRequest.RequestID,
		Key:       dataRequest.Key,
		Value:     value,
	}

	msgBytes, err := n.conf.MessageRegistry.MarshalMessage(dataReply)

	if err != nil {
		log.Error().Msg("Error marshaling data reply message")
		return err
	}

	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		pkt.Header.Source,
	)

	//! send data reply message

	replyPkt := createTransportPacket(&header, &msgBytes)

	nextHop, err := n.getNextHop(pkt.Header.Source)

	if err != nil {
		return err
	}

	err = n.conf.Socket.Send(nextHop, replyPkt, time.Second*1)
	if err != nil {
		log.Error().Msgf("Error sending data reply message to %s", pkt.Header.Source)
		return err
	}

	n.handledDataRequests.Add(dataReply.RequestID, pkt.Header.Source)

	return nil
}

func (n *node) handleDataReplyMessage(msg types.Message, pkt transport.Packet) error {

	log := n.getLogger()

	dataReply, ok := msg.(*types.DataReplyMessage)

	if !ok {
		log.Error().Msgf("Expected data reply message, got %T", dataReply)
	}

	value, ok := n.dataReplyChan.Get(dataReply.RequestID)

	if ok {
		//notification
		replyChan := value

		select {
		case replyChan <- dataReply:
			// Successfully sent the reply to the waiting goroutine
		default:
			// Channel is full or closed
			log.Warn().Msg("Reply channel is full or closed")
		}
	} else {
		log.Warn().Msgf("No waiting request for RequestID %s", dataReply.RequestID)
	}

	return nil
}

func (n *node) forwardSearchRequest(searchReq types.SearchRequestMessage, source string) error {
	if searchReq.Budget > 1 {
		remBudget := searchReq.Budget - 1
		neighbors := n.getOtherNeighbors(source)

		numNeighbors := len(neighbors)

		if numNeighbors > 0 {
			budgetAllocation := n.divideBudget(uint64(remBudget), uint64(numNeighbors))

			rand.Shuffle(len(neighbors), func(i, j int) {
				neighbors[i], neighbors[j] = neighbors[j], neighbors[i]
			})

			//forward the search request for as long as the budget is not 0
			for i, neighbor := range neighbors {
				neighborBudget := budgetAllocation[i]

				if neighborBudget > 0 {
					n.sendForwardedSearchRequest(&searchReq, neighbor, uint(neighborBudget))

				}
			}
		}
	}
	return nil
}

func (n *node) handleSearchRequestMessage(msg types.Message, pkt transport.Packet) error {

	log := n.getLogger()
	searchReq, ok := msg.(*types.SearchRequestMessage)
	if n.handledSearchRequests.Exists(searchReq.RequestID) {
		return nil
	}

	if !ok {
		log.Error().Msgf("Expected SearchReq Message, got %T", msg)
		return xerrors.Errorf("Unexcepted message type: %T", msg)
	}

	//!Forward the search request

	fwResult := n.forwardSearchRequest(*searchReq, pkt.Header.Source)

	if fwResult != nil {
		log.Error().Msg("Failed to forward search request")
		return fwResult
	}

	//! search the data in node storage
	reg := regexp.MustCompile(searchReq.Pattern)
	matchingFiles := n.SearchLocalFiles(reg)

	//! send reply message
	log.Info().Msgf("Sending reply message to %s", pkt.Header.Source)

	sendResult := n.sendReplyMessage(matchingFiles, searchReq.RequestID, searchReq.Origin, pkt.Header.Source)

	if sendResult != nil {
		log.Error().Msg("Failed to send reply message")
		return sendResult
	}

	n.handledSearchRequests.Add(searchReq.RequestID, pkt.Header.Source)

	return nil
}

func (n *node) handleSearchReplyMessage(msg types.Message, pkt transport.Packet) error {

	log := n.getLogger()

	searchReply, ok := msg.(*types.SearchReplyMessage)

	if !ok {
		log.Error().Msgf("Expected SearchReplyMessage, got %T", msg)
		return xerrors.Errorf("Expected SearchReplyMessage, got %T", msg)
	}

	//! Update the catalog

	namingStore := n.conf.Storage.GetNamingStore()

	for _, fileInfo := range searchReply.Responses {
		n.catalog.Update(fileInfo.Metahash, pkt.Header.Source, n.conf.Socket.GetAddress())
		namingStore.Set(fileInfo.Name, []byte(fileInfo.Metahash))

		for _, chunk := range fileInfo.Chunks {
			log.Info().Msgf("Updating catalog with chunk %s", chunk)
			if chunk != nil {
				n.catalog.Update(string(chunk), pkt.Header.Source, n.conf.Socket.GetAddress())
			}
		}

	}

	value, ok := n.searchReplyChan.Get(searchReply.RequestID)

	if !ok {
		log.Warn().Msgf("No waiting request for RequestID %s", searchReply.RequestID)
		return nil
	}

	replyChan := value

	select {
	case replyChan <- searchReply:
		// Successfully sent the reply to the waiting goroutine
		log.Debug().Msgf("Sent SearchReplyMessage with RequestID %s to waiting goroutine", searchReply.RequestID)
	default:
		// Channel is full or closed
		log.Warn().Msg("Reply channel is full or closed")
	}

	return nil

}

// Begin New Handlers for DNS messages

func (n *node) handleDNSReadMessage(msg types.Message, pkt transport.Packet) error {
	// Handle DNS read message
	DNSReadRequest, ok := msg.(*types.DNSReadMessage)
	if !ok {
		log.Error().Msgf("Expected DNSReadMessage, got %T", msg)
		return xerrors.Errorf("Expected DNSReadMessage, got %T", msg)
	}

	DNSReadEntry, success := n.dnsStore.Get(DNSReadRequest.Domain)
	if !success {
		log.Error().Msgf("Domain %s not found", DNSReadRequest.Domain)
		return xerrors.Errorf("Domain %s not found", DNSReadRequest.Domain)
	}

	// Calculate TTL based on expiration time
	ttl := time.Until(DNSReadEntry.Expiration)
	if ttl < 0 {
		ttl = 0
	}

	DNSReadReply := &types.DNSReadReplyMessage{
		Domain:    DNSReadEntry.Domain,
		IPAddress: DNSReadEntry.IPAddress,
		TTL:       ttl,
	}

	// Send DNSReadReply to the source
	msgBytes, err := n.conf.MessageRegistry.MarshalMessage(DNSReadReply)
	if err != nil {
		log.Error().Msgf("Failed to marshal DNSReadReply message: %v", err)
		return err
	}

	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		pkt.Header.Source,
	)

	replyPkt := createTransportPacket(&header, &msgBytes)

	err = n.conf.Socket.Send(pkt.Header.Source, replyPkt, time.Second*1)
	if err != nil {
		log.Error().Msgf("Failed to send DNSReadReply message: %v", err)
		return err
	}

	return nil
}

func (n *node) handleDNSRenewalMessage(msg types.Message, pkt transport.Packet) error {
	// Handle DNS renewal message
	DNSRenewalRequest, ok := msg.(*types.DNSRenewalMessage)
	if !ok {
		log.Error().Msgf("Expected DNSRenewalMessage, got %T", msg)
		return xerrors.Errorf("Expected DNSRenewalMessage, got %T", msg)
	}

	DNSReadEntry, success := n.dnsStore.Get(DNSRenewalRequest.Domain)
	if !success {
		log.Error().Msgf("Domain %s not found", DNSRenewalRequest.Domain)
		return xerrors.Errorf("Domain %s not found", DNSRenewalRequest.Domain)
	}

	// Update the expiration time
	DNSReadEntry.Expiration = DNSRenewalRequest.Expiration
	n.dnsStore.Add(DNSRenewalRequest.Domain, DNSReadEntry)

	log.Info().Msgf("Domain %s renewed until %s", DNSRenewalRequest.Domain, DNSRenewalRequest.Expiration)
	return nil
}

func (n *node) handleDNSRegisterMessage(msg types.Message, pkt transport.Packet) error {
	// Handle DNS register message
	DNSRegisterRequest, ok := msg.(*types.DNSRegisterMessage)
	if !ok {
		log.Error().Msgf("Expected DNSRegisterMessage, got %T", msg)
		return xerrors.Errorf("Expected DNSRegisterMessage, got %T", msg)
	}

	// Create a new DNS entry
	DNSReadEntry := peer.DNSEntry{
		Domain:     DNSRegisterRequest.Domain,
		IPAddress:  DNSRegisterRequest.IPAddress,
		Expiration: DNSRegisterRequest.Expiration,
	}

	n.dnsStore.Add(DNSRegisterRequest.Domain, DNSReadEntry)

	log.Info().Msgf("Domain %s registered with IP %s until %s", DNSRegisterRequest.Domain, DNSRegisterRequest.IPAddress, DNSRegisterRequest.Expiration)
	return nil
}

func (n *node) handleDNSReadReplyMessage(msg types.Message, pkt transport.Packet) error {
	// Handle DNS read reply message
	DNSReadReply, ok := msg.(*types.DNSReadReplyMessage)
	if !ok {
		log := n.getLogger()
		log.Error().Msgf("Expected DNSReadReplyMessage, got %T", msg)
		return xerrors.Errorf("Expected DNSReadReplyMessage, got %T", msg)
	}

	log := n.getLogger()
	log.Info().
		Str("domain", DNSReadReply.Domain).
		Str("ip_address", DNSReadReply.IPAddress).
		Dur("ttl", DNSReadReply.TTL).
		Msg("Received DNS read reply message")

	// Process the DNS read reply message as needed
	// For example, you might want to update some internal state or notify other components

	return nil
}
