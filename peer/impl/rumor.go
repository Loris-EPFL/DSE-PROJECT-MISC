package impl

import (
	"sort"
	"time"

	"github.com/rs/zerolog/log"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

//function that processes a rumor
func (n* node) processRumorMessage(rumor types.Rumor, pkt transport.Packet) error {


    //Store the rumor in the rumor db
    n.storeRumor(rumor)

    newPkt := createTransportPacket(pkt.Header, rumor.Msg)


    err := n.conf.MessageRegistry.ProcessPacket(newPkt)
    if err != nil {
        return xerrors.Errorf("failed to process embedded message: %v", err)
    }
    return nil
}




//function that stores the received or created rumors in the rumor database

func (n *node) storeRumor(rumor types.Rumor){




    origin := rumor.Origin
    rumorsByOrigin, exists := n.rumorDB.Get(origin)

    if !exists {
        rumorsByOrigin = types.RumorByOrigin{}
    }

    // Insert rumor in order
    index := sort.Search(len(rumorsByOrigin), func(i int) bool {
        return rumorsByOrigin[i].Sequence >= rumor.Sequence
    })

    if index < len(rumorsByOrigin) && rumorsByOrigin[index].Sequence == rumor.Sequence {
        return
    }

    // Insert the rumor at the found index
    rumorsByOrigin = append(rumorsByOrigin, types.Rumor{})
    copy(rumorsByOrigin[index+1:], rumorsByOrigin[index:])
    rumorsByOrigin[index] = rumor

    n.rumorDB.Add(origin, rumorsByOrigin)

}





//Returns the missing rumors for a given origin between startSeq and endSeq
func (n *node) getMissingRumors(origin string, startSeq , endSeq uint) []types.Rumor{




    rumorsByOrigin, exists := n.rumorDB.Get(origin)

    if !exists{
        return nil
    }
    var missingRumors []types.Rumor


    //Compare seq numbers so copy all the rumors from remoteSeq+1 up to localSeq
    for _, rumor := range rumorsByOrigin {
        if rumor.Sequence < startSeq{
            continue
        }
        if rumor.Sequence > endSeq{
            break
        }
        missingRumors = append(missingRumors, rumor)
    }
    return missingRumors
}   





//function that sends missing rumors to a remote peer
func (n *node) sendMissingRumors(source string, missing map[string][]types.Rumor) error {
    
    log := n.getLogger()
    var rumorsList []types.Rumor


    for _, rumors := range missing {
        rumorsList = append(rumorsList, rumors...)
    }

    if len(rumorsList) == 0 {
        return xerrors.Errorf("No rumor to send")
    }
    
    //Prepare the rumors in a packet 
    rumorsMessage := types.RumorsMessage{
        Rumors: rumorsList,
    }
    rumorsMsgTransp, err := n.conf.MessageRegistry.MarshalMessage(rumorsMessage)
    if err != nil {
        log.Error().Msgf("Failed to marshal RumorsMessage")
        return xerrors.Errorf("Failed to marshal RumorsMessage")
    }
    header := transport.NewHeader(
        n.conf.Socket.GetAddress(),
        n.conf.Socket.GetAddress(),
        source,
    )
    pkt := createTransportPacket(&header, &rumorsMsgTransp)

    //Send the missing rumors
    err = n.conf.Socket.Send(source, pkt, time.Second *1)
    if err != nil{
        log.Error().Msgf("Failed to send RumorsMessage to %s", source)
        return xerrors.Errorf("Failed to send RumorsMessage to %s", source)
    }

    return nil
}




//function that forwards a rumor to a random neighbour
func (n *node) forwardRumorsMessage(rumors *types.RumorsMessage, receivedPkt transport.Packet) error {
 
    otherNeighbors := n.getOtherNeighbors(receivedPkt.Header.Source)
   
    if len(otherNeighbors) == 0 {
        // No other neighbors to forward to
        return nil
    }

    randNeighbor := getRandom(otherNeighbors)
   
    rumorMsgTransp, err := n.conf.MessageRegistry.MarshalMessage(rumors)
   
    if err!= nil {
        log.Error().Msgf("Failed to marshal %s: %v", rumors, err)
        return err
    }
   
    fwHeader := transport.NewHeader(
        n.conf.Socket.GetAddress(),
        n.conf.Socket.GetAddress(),
        randNeighbor,
    )
    fwPkt := createTransportPacket(&fwHeader, &rumorMsgTransp)
   
   
    // Send the packet
    err = n.conf.Socket.Send(randNeighbor, fwPkt, time.Second*1)
    if err != nil {
        log.Error().Msgf("Failed to forward packet %s to %s: %v", fwPkt, randNeighbor, err )
        return err
    }
    
    go n.waitForAck(fwPkt, randNeighbor)
   
    return nil
   }