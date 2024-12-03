package impl

import (
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

func (n *node) continueMongering(source string){

    if rand.Float64() <= n.conf.ContinueMongering {
        neighbors := n.getOtherNeighbors(source)

        if len(neighbors) == 0{
            return
        }
        randNeighbor := getRandom(neighbors)
        err := n.sendStatusMessage(randNeighbor)
        if err != nil {
            log.Error().
                Msgf("Impossible to send status message from %s to %s", 
                    n.conf.Socket.GetAddress(), 
                    randNeighbor)
        } 
    }
}


// Function that waits for an ack of a given peer
func (n *node) waitForAck(rumorPkt transport.Packet, neighbor string) {

    log := n.getLogger()


    
    if n.conf.AckTimeout == 0 {
        return 
    }

    packetID := rumorPkt.Header.PacketID
    ackTimeout := n.conf.AckTimeout

    ackReceived := make(chan *types.AckMessage, 1)

    log.Warn().Msgf("Added channel for ack %s", packetID)

    n.ackChannel.Add(packetID, ackReceived)
    

        for {
            select {
                case <-n.stopCh:
                    log.Info().Msg("Stopping waitForAck")

                    return

                case ackMsg := <- ackReceived:
                    if ackMsg.AckedPacketID == packetID {
                        log.Info().Msgf("Ack received for PacketID %s", packetID)
                        n.ackChannel.Remove(packetID)
                        close(ackReceived)
                        return 
                    }

                case <-time.After(ackTimeout):
                    //Timeout Ocurred: Need to send rumor to another peer
                    log.Warn().Msgf("Ack timeout for PacketID %s", packetID)
                    n.ackChannel.Remove(packetID)
                    close(ackReceived)
                    //get other neighbor
                    otherNeighbors := n.getOtherNeighbors(neighbor)

                    if len(otherNeighbors) == 0{
                        log.Warn().Msg("No other neighbors to resend rumor")
                    }

                    //get another random neighbor
                    randNeighbor := getRandom(otherNeighbors)

                    //create header
                    newHeader := transport.NewHeader(
                        n.conf.Socket.GetAddress(),
                        n.conf.Socket.GetAddress(),
                        randNeighbor,
                    )

                    newPkt := createTransportPacket(&newHeader, rumorPkt.Msg)

                    log.Debug().Msgf("Sending rumor to %s", randNeighbor)

                    if err := n.conf.Socket.Send(randNeighbor, newPkt, time.Second * 1); err != nil {
                        log.Error().
                            Str("destination", randNeighbor).
                            Err(err).
                            Msg("Failed to resend rumor message")
                        return 
                    }
                    go n.waitForAck(newPkt, randNeighbor)
                    log.Debug().
                        Msgf("Resent rumor to %s with PacketID %s", randNeighbor, newPkt.Header.PacketID)

                     

                    return

                }
                
        }
     
}


//Function that sends an Ack when a rumor is received 
func(n *node) sendAckMessage(pkt transport.Packet) error {
        statusMsg := n.createStatusMessage()
    
        ackMsg := types.AckMessage{
            AckedPacketID: pkt.Header.PacketID,
            Status: statusMsg,
        }
    
        // Marshal the AckMessage
        ackMsgTransp, err := n.conf.MessageRegistry.MarshalMessage(&ackMsg)
        if err != nil {
            return xerrors.Errorf("failed to marshal AckMessage: %v", err)
        }
    
        ackHeader := transport.NewHeader(
            n.conf.Socket.GetAddress(), 
            n.conf.Socket.GetAddress(), 
            pkt.Header.Source,          
        )
    
        // Create the packet
    
        ackPkt := createTransportPacket(&ackHeader, &ackMsgTransp)
        
    
        // Send the AckMessage back to the source
        err = n.conf.Socket.Send(pkt.Header.Source, ackPkt, time.Second*1)
        if err != nil {
            return xerrors.Errorf("failed to send AckMessage: %v", err)
        }
        return nil
    }



//Function that compares two views and returns in which of the 4 
//cases we are, and also returs the missing rumors (to be sent by node n)
func (n *node) compareViews(view types.StatusMessage) (uint, map[string][]types.Rumor){




    //if true then local has new rumors
    newLocal := false

    //if true then remote has new rumors
    newRemote := false

    missing := make(map[string][]types.Rumor)


    for origin, remoteSeq := range view {
        localSeq, localExists := n.receivedSeq.Get(origin)
        // log.Info().Msgf("origin: %s", origin)
        // log.Info().Msgf("localSeq: %d", localSeq)
        // log.Info().Msgf("localexists: %t", localExists)


 

        switch {
            //Remote has rumors that we don't know from other origins
        case !localExists && remoteSeq > 0:
            newRemote = true
            //Remote has rumors that we don't know from this origin
        case remoteSeq > localSeq:
            newRemote = true
            //I have rumors that he doesn't have
        case localSeq > remoteSeq:
            newLocal = true
            missing[origin] = n.getMissingRumors(origin, remoteSeq+1, localSeq)
        }
    }
    

    copyMap := n.receivedSeq.ToMap()

    for origin, localSeq := range copyMap {
        if _, remoteExists := view[origin]; !remoteExists {
            newLocal = true
            missing[origin] = n.getMissingRumors(origin, 1, localSeq)
        }
    }

        //Chose the actual case
        switch {
        case newRemote && newLocal:
            return 3, missing
        case newRemote:    
            return 1, nil
        case newLocal: 
            return 2, missing
        default:
            return 4, nil
        }
}



//function that sends a status message to the sender of the previous one
func (n *node) sendStatusMessage(destination string) error {
    log := n.getLogger()


    //Create the message and prepare it to send
    statusMsg := n.createStatusMessage()
    statusMsgTransp, err := n.conf.MessageRegistry.MarshalMessage(statusMsg)
    if err != nil {
        return xerrors.Errorf("Failed to Marshal Status Message")
    }
    header := transport.NewHeader(
        n.conf.Socket.GetAddress(),
        n.conf.Socket.GetAddress(),
        destination,
    )
    pkt := transport.Packet{
        Header: &header,
        Msg: &statusMsgTransp,
    }

    //Send the packet containing the status message
    err = n.conf.Socket.Send(destination, pkt, time.Second * 1)

    if err != nil {
        return xerrors.Errorf("Failed to send status message to %s, got %T", destination, err)
    }

    log.Info().Msgf("Status message sent to %s", destination)

    return nil

}


func (n *node) createStatusMessage() types.StatusMessage {

    status := make(map[string]uint)

    copyMap := n.receivedSeq.ToMap()

    for origin, seq := range copyMap{
        status[origin] = seq
    }


    
 return status
}