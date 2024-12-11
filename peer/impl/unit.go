package impl

import (
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/channel"
	"go.dedis.ch/cs438/transport/udp"
)

// This file is copied from the unit.go existing in provided unit tests folders.

var peerFac peer.Factory = NewPeer

var channelFac transport.Factory = channel.NewTransport
var udpFac transport.Factory = udp.NewUDP
