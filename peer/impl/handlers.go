package impl

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
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
		With().Str("role", "handlers.go").Logger()
}

// auxiliary function that handles incoming packets
func (n *node) handleIncomingPacket() error {

	pkt, err := n.conf.Socket.Recv(time.Second)
	if err != nil {
		return err
	}

	logger.Info().
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

	privMsg, ok := msg.(*types.PrivateMessage)

	if !ok {
		logger.Error().
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
		logger.Info().Msg("Processing PrivateMessage")

		privPkt := createTransportPacket(pkt.Header, privMsg.Msg)

		err := n.conf.MessageRegistry.ProcessPacket(privPkt)

		if err != nil {
			logger.Error().Err(err).Msg("Failed to process embedded message")
			return xerrors.Errorf("Failed to process embedded message")
		}
	} else {
		logger.Info().Msg("Not a recipient of PrivateMessage; ignoring")

	}

	return nil
}

func (n *node) handleEmptyMessage(msg types.Message, pkt transport.Packet) error {
	return nil
}

func (n *node) handleChatMessage(msg types.Message, pkt transport.Packet) error {

	chatMsg, ok := msg.(*types.ChatMessage)

	if !ok {
		logger.Error().
			Msgf("Expected ChatMessage, got %T", msg)
		return xerrors.Errorf("expected ChatMessage, got %T", msg)
	}

	logger.Info().
		Str("message", chatMsg.Message).
		Msg("Received chat message")

	return nil
}

func (n *node) handleStatusMessage(msg types.Message, pkt transport.Packet) error {

	statusMsg, ok := msg.(*types.StatusMessage)

	if !ok {
		logger.Error().
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
			logger.Error().Msgf("Impossible to send status message to %s", pkt.Header.Source)
		}

	//2. local has views that remote hasn't
	case 2:
		err := n.sendMissingRumors(pkt.Header.Source, rumorsToSend)
		if err != nil {
			logger.Error().Msgf("Impossible to send missing rumors to to %s", pkt.Header.Source)
		}

	//3. Both have views that the other hasn't
	case 3:
		errStat := n.sendStatusMessage(pkt.Header.Source)

		errMiss := n.sendMissingRumors(pkt.Header.Source, rumorsToSend)

		if errStat != nil || errMiss != nil {
			logger.Error().
				Msgf("Unable to send status or missing rumors to %s", pkt.Header.Source)

		}
	//4. Same view
	case 4:
		n.continueMongering(pkt.Header.Source)
	}

	return nil
}

func (n *node) handleAckMessage(msg types.Message, pkt transport.Packet) error {

	ackMsg, ok := msg.(*types.AckMessage)

	if !ok {
		logger.Error().
			Msgf("Expected AckMessage, got %T", msg)
		return xerrors.Errorf("Expected AckMessage got %T", msg)
	}

	goodChan, exists := n.ackChannel.Get(ackMsg.AckedPacketID)

	if !exists {
		logger.Error().Msgf("No acked packet ID found")
	} else {
		goodChan <- ackMsg
	}

	logger.Warn().Msgf("Channel %v", goodChan)

	statusMsg := ackMsg.Status

	statusMsgTransp, err := n.conf.MessageRegistry.MarshalMessage(statusMsg)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal StatusMessage")
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

	rumors, ok := msg.(*types.RumorsMessage)

	if !ok {
		logger.Error().Msgf("Expected rumor, got %T", msg)
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
				logger.Error().
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
				logger.Error().
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
			logger.Error().
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
		logger.Error().Msg("Error marshaling data reply message")
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
		logger.Error().Msgf("Error sending data reply message to %s", pkt.Header.Source)
		return err
	}

	n.handledDataRequests.Add(dataReply.RequestID, pkt.Header.Source)

	return nil
}

func (n *node) handleDataReplyMessage(msg types.Message, pkt transport.Packet) error {

	dataReply, ok := msg.(*types.DataReplyMessage)

	if !ok {
		logger.Error().Msgf("Expected data reply message, got %T", dataReply)
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
			logger.Warn().Msg("Reply channel is full or closed")
		}
	} else {
		logger.Warn().Msgf("No waiting request for RequestID %s", dataReply.RequestID)
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

	searchReq, ok := msg.(*types.SearchRequestMessage)
	if n.handledSearchRequests.Exists(searchReq.RequestID) {
		return nil
	}

	if !ok {
		logger.Error().Msgf("Expected SearchReq Message, got %T", msg)
		return xerrors.Errorf("Unexcepted message type: %T", msg)
	}

	//!Forward the search request

	fwResult := n.forwardSearchRequest(*searchReq, pkt.Header.Source)

	if fwResult != nil {
		logger.Error().Msg("Failed to forward search request")
		return fwResult
	}

	//! search the data in node storage
	reg := regexp.MustCompile(searchReq.Pattern)
	matchingFiles := n.SearchLocalFiles(reg)

	//! send reply message
	logger.Info().Msgf("Sending reply message to %s", pkt.Header.Source)

	sendResult := n.sendReplyMessage(matchingFiles, searchReq.RequestID, searchReq.Origin, pkt.Header.Source)

	if sendResult != nil {
		logger.Error().Msg("Failed to send reply message")
		return sendResult
	}

	n.handledSearchRequests.Add(searchReq.RequestID, pkt.Header.Source)

	return nil
}

func (n *node) handleSearchReplyMessage(msg types.Message, pkt transport.Packet) error {

	searchReply, ok := msg.(*types.SearchReplyMessage)

	if !ok {
		logger.Error().Msgf("Expected SearchReplyMessage, got %T", msg)
		return xerrors.Errorf("Expected SearchReplyMessage, got %T", msg)
	}

	//! Update the catalog

	namingStore := n.conf.Storage.GetNamingStore()

	for _, fileInfo := range searchReply.Responses {
		n.catalog.Update(fileInfo.Metahash, pkt.Header.Source, n.conf.Socket.GetAddress())
		namingStore.Set(fileInfo.Name, []byte(fileInfo.Metahash))

		for _, chunk := range fileInfo.Chunks {
			logger.Info().Msgf("Updating catalog with chunk %s", chunk)
			if chunk != nil {
				n.catalog.Update(string(chunk), pkt.Header.Source, n.conf.Socket.GetAddress())
			}
		}

	}

	value, ok := n.searchReplyChan.Get(searchReply.RequestID)

	if !ok {
		logger.Warn().Msgf("No waiting request for RequestID %s", searchReply.RequestID)
		return nil
	}

	replyChan := value

	select {
	case replyChan <- searchReply:
		// Successfully sent the reply to the waiting goroutine
		logger.Debug().Msgf("Sent SearchReplyMessage with RequestID %s to waiting goroutine", searchReply.RequestID)
	default:
		// Channel is full or closed
		logger.Warn().Msg("Reply channel is full or closed")
	}

	return nil

}

// Begin New Handlers for DNS messages

// handleDNSReadRequestMessage processes a DNS read request and sends a reply with the DNS entry details.
func (n *node) handleDNSReadRequestMessage(msg types.Message, pkt transport.Packet) error {
	// Handle DNS read message
	DNSReadRequest, ok := msg.(*types.DNSReadRequestMessage)
	logger.Info().Any("request", DNSReadRequest).Msg("Received DNS read request message")
	if !ok {
		logger.Error().Msgf("Expected DNSReadRequestMessage, got %T", msg)
		return xerrors.Errorf("Expected DNSReadRequestMessage, got %T", msg)
	}

	// Check if the domain exists in the DNS store
	UTXO, ok := n.UTXOSet.Get(DNSReadRequest.Domain)
	logger.Info().Any("UTXO", n.UTXOSet.ToMap()).Msg("UTXO Set in Read handler")
	if !ok {
		logger.Error().Msgf("Domain %s not found", DNSReadRequest.Domain)
		return xerrors.Errorf("Domain %s not found", DNSReadRequest.Domain)
	}

	DNSReadReply := &types.DNSReadReplyMessage{
		Domain:     UTXO.DomainName,
		IPAddress:  UTXO.IP,
		Owner:      UTXO.Owner,
		Exists:     true,
		Expiration: UTXO.Expiration,
	}

	// Send DNSReadReply to the source
	msgBytes, err := n.conf.MessageRegistry.MarshalMessage(DNSReadReply)
	if err != nil {
		logger.Error().Msgf("Failed to marshal DNSReadReply message: %v", err)
		return err
	}

	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		pkt.Header.Source,
	)

	replyPkt := createTransportPacket(&header, &msgBytes)

	// Send the DNSReadReply message
	logger.Info().Any("pkt", replyPkt).Msg("Sending DNS read reply message")

	err = n.conf.Socket.Send(pkt.Header.Source, replyPkt, time.Second*1)
	if err != nil {
		logger.Error().Msgf("Failed to send DNSReadReply message: %v", err)
		return err
	}

	return nil
}

func (n *node) handleDNSReadReplyMessage(msg types.Message, pkt transport.Packet) error {
	// Handle DNS read reply message
	DNSReadReply, ok := msg.(*types.DNSReadReplyMessage)
	if !ok {

		logger.Error().Msgf("Expected DNSReadReplyMessage, got %T", msg)
		return xerrors.Errorf("Expected DNSReadReplyMessage, got %T", msg)
	}

	logger.Info().
		Str("domain", DNSReadReply.Domain).
		Str("ip_address", DNSReadReply.IPAddress).
		Dur("ttl", DNSReadReply.TTL).
		Msg("Received DNS read reply message")

	// Process the DNS read reply message as needed
	// For example, you might want to update some internal state or notify other components

	return nil
}

// Added Handler for TransactionMessage (can be new, firstupdate or update)
func (n *node) handleTransactionMessage(msg types.Message, pkt transport.Packet) error {

	txMsg, ok := msg.(*types.TransactionMessage)
	if !ok {
		return fmt.Errorf("expected TransactionMessage, got %T", msg)
	}

	// Validate transaction fields (syntactic checks, etc.)
	err := n.validateTransaction(&txMsg.Tx)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid transaction received")
		return err
	}

	// If valid, add to mempool
	n.mempool.Add(txMsg.Tx.ID, txMsg.Tx)
	logger.Info().Msgf("Received transaction %s and added it to mempool", txMsg.Tx.ID)

	n.newTxCh <- struct{}{}

	logger.Info().Msg("New transaction received, notifying mining routine")

	return nil
}

func (n *node) handleBlockMessage(msg types.Message, pkt transport.Packet) error {

	blockMsg, ok := msg.(*types.BlockMessage)

	if !ok {
		return xerrors.Errorf("expected BlockMessage, got %T", msg)
	}

	// Validate the block
	err := n.validateBlock(&blockMsg.Block)
	if err != nil {
		logger.Error().Err(err).Msg("Invalid block received")
		return err
	}

	// Add the block to the blockchain
	err = n.addBlockToBlockchain(&blockMsg.Block)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to add block to blockchain")
		return err
	}

	n.removeTxsFromMempool(blockMsg.Block.Transactions)
	logger.Info().Msgf("Received block %d and added it to the blockchain", blockMsg.Block.Nonce)
	logger.Info().Msgf("I have %d blocks in the blockchain", n.getCurrentNonce())

	logger.Info().Msgf("UTXO Set: %v", n.UTXOSet.ToMap())

	return nil

}
