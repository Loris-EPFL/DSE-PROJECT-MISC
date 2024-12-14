package unit

import (
	"crypto/sha256"
	"encoding/hex"

	// "encoding/json"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/transport/channel"
	"go.dedis.ch/cs438/types"
)

var (
	defaultLevel = zerolog.InfoLevel
	logout       = zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
	logger zerolog.Logger
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
		With().Str("role", "Test File").Logger()
}

// Test handleDNSReadMessage
func Test_HandleDNSReadMessage(t *testing.T) {
	transp := channel.NewTransport()

	//Create 2 nodes and a reader (sender type since he "propose" a readDNSRequest)
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithPowBits(3))
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.2:0", z.WithPowBits(3))
	defer node2.Stop()

	proposer, err := z.NewSenderSocket(transp, "127.0.0.3:0")
	require.NoError(t, err)

	reader, err := z.NewSenderSocket(transp, "127.0.0.4:0")
	require.NoError(t, err)

	node1.AddPeer(node2.GetAddr())
	node2.AddPeer(node1.GetAddr())

	// Register a DNS entry
	dnsEntry := types.DNSReadRequestMessage{
		Domain: "example.com",
		TTL:    time.Second,
	}

	//Fields for the DNS entry inside the block
	IPAddress := "192.168.1.1"
	Expiration := time.Now().Add(time.Hour)
	Owner := "owner1"

	// Create NameNew transaction
	hash := sha256.New()
	hash.Write([]byte("random_salt" + dnsEntry.Domain))
	saltedHash := hex.EncodeToString(hash.Sum(nil))

	//Create NameNew transaction
	nameNewTx := types.Transaction{
		ID:           "tx1",
		Type:         types.NameNew,
		Input:        types.UTXO{}, // Dummy input
		Output:       types.UTXO{DomainName: saltedHash, IP: IPAddress, Owner: Owner, Expiration: Expiration},
		HashedDomain: saltedHash,
		Fees:         1,
	}

	msg := types.TransactionMessage{
		Tx: nameNewTx,
	}
	transpStatusMsg, err := node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node1.GetAddr())

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameNew transaction
	err = proposer.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	//Wait 3 seconds, it should instead be wait for about 3 blocks to be mined
	time.Sleep(time.Second * 3)

	// Create NameFirstUpdate transaction
	nameFirstUpdateTx := types.Transaction{
		ID:          "tx2",
		Type:        types.NameFirstUpdate,
		Input:       nameNewTx.Output, // Dummy input
		Output:      types.UTXO{DomainName: dnsEntry.Domain, IP: IPAddress, Owner: Owner, Expiration: Expiration},
		PlainDomain: dnsEntry.Domain,
		Salt:        "random_salt",
		Fees:        1,
	}

	msg = types.TransactionMessage{
		Tx: nameFirstUpdateTx,
	}
	transpStatusMsg, err = node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header = transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node2.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameFirstUpdate transaction
	err = proposer.Send(node2.GetAddr(), packet, time.Second*1)

	require.NoError(t, err)

	//Wait 1 seconds, it should not be needed since we should right after the NameFirstUpdate
	time.Sleep(time.Second * 1)

	transpRequestMsg, err := node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header = transport.NewHeader(reader.GetAddress(), reader.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpRequestMsg,
	}

	// Send DNSReadRequestMessage
	err = reader.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	//Check if DNSReadReplyMessage was received
	ins := node1.GetIns()
	//logger.Info().Any("ins", ins).Msg("Received messages")

	//Check if one of the message received is a DNSReadRequestMessage
	hasDNSReadRequest := false
	for _, in := range ins {
		if in.Msg.Type == "DNSReadRequestMessage" {
			request := &types.DNSReadRequestMessage{}
			err := node1.GetRegistry().UnmarshalMessage(in.Msg, request)
			require.NoError(t, err)
			logger.Info().Any("received request", request).Msg("test DNSReadRequestMessage")
			require.Equal(t, request.Domain, dnsEntry.Domain)
			require.Equal(t, request.TTL, dnsEntry.TTL)
			hasDNSReadRequest = true
			break
		}
	}

	require.True(t, hasDNSReadRequest)

	outs := node1.GetOuts()
	//Fliter out DNSReadReplyMessage, find the one that matches the domain, check if all fields are respected
	for _, out := range outs {
		if out.Msg.Type == "DNSReadReplyMessage" {
			reply := &types.DNSReadReplyMessage{}
			err := node1.GetRegistry().UnmarshalMessage(out.Msg, reply)
			require.NoError(t, err)
			logger.Info().Any("reply", reply).Msg("test DNSReadReplyMessage")
			require.Equal(t, reply.Domain, dnsEntry.Domain)
			require.Equal(t, reply.IPAddress, IPAddress)
			require.LessOrEqual(t, reply.Expiration, Expiration) //LessOrEqual because of imprecision error, but should virtually be the same
			require.Equal(t, reply.Owner, Owner)
			break
		}
	}

}

// Test handleDNSReadMessage
func Test_HandleDNSUpdateScenario(t *testing.T) {
	transp := channel.NewTransport()

	//Create 2 nodes and a reader (sender type since he "propose" a readDNSRequest)
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithPowBits(3))
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.2:0", z.WithPowBits(3))
	defer node2.Stop()

	proposer, err := z.NewSenderSocket(transp, "127.0.0.3:0")

	require.NoError(t, err)

	reader, err := z.NewSenderSocket(transp, "127.0.0.4:0")
	require.NoError(t, err)

	node1.AddPeer(node2.GetAddr())
	node2.AddPeer(node1.GetAddr())

	// Register a DNS entry
	dnsEntry := types.DNSReadRequestMessage{
		Domain: "example.com",
		TTL:    time.Second,
	}

	//Fields for the DNS entry inside the block
	IPAddress := "192.168.1.1"
	Expiration := time.Now().Add(time.Hour)
	Owner := "owner1"

	// Create NameNew transaction
	hash := sha256.New()
	hash.Write([]byte("random_salt" + dnsEntry.Domain))
	saltedHash := hex.EncodeToString(hash.Sum(nil))

	//Create NameNew transaction
	nameNewTx := types.Transaction{
		ID:           "tx1",
		Type:         types.NameNew,
		Input:        types.UTXO{}, // Dummy input
		Output:       types.UTXO{DomainName: saltedHash, IP: IPAddress, Owner: Owner, Expiration: Expiration},
		HashedDomain: saltedHash,
		Fees:         1,
	}

	msg := types.TransactionMessage{
		Tx: nameNewTx,
	}
	transpStatusMsg, err := node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node1.GetAddr())

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameNew transaction
	err = proposer.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	//Wait 3 seconds, it should instead be wait for about 3 blocks to be mined
	time.Sleep(time.Second * 3)

	// Create NameFirstUpdate transaction
	nameFirstUpdateTx := types.Transaction{
		ID:          "tx2",
		Type:        types.NameFirstUpdate,
		Input:       nameNewTx.Output, // Dummy input
		Output:      types.UTXO{DomainName: dnsEntry.Domain, IP: IPAddress, Owner: Owner, Expiration: Expiration},
		PlainDomain: dnsEntry.Domain,
		Salt:        "random_salt",
		Fees:        1,
	}

	msg = types.TransactionMessage{
		Tx: nameFirstUpdateTx,
	}
	transpStatusMsg, err = node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header = transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node2.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameFirstUpdate transaction
	err = proposer.Send(node2.GetAddr(), packet, time.Second*1)

	require.NoError(t, err)

	//Wait 1 seconds, it should not be needed since we should right after the NameFirstUpdate
	time.Sleep(time.Second * 1)

	transpRequestMsg, err := node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header = transport.NewHeader(reader.GetAddress(), reader.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpRequestMsg,
	}

	// Send DNSReadRequestMessage
	err = reader.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	//Check if DNSReadReplyMessage was received
	ins := node1.GetIns()
	//logger.Info().Any("ins", ins).Msg("Received messages")

	//Check if one of the message received is a DNSReadRequestMessage
	hasDNSReadRequest := false
	for _, in := range ins {
		if in.Msg.Type == "DNSReadRequestMessage" {
			request := &types.DNSReadRequestMessage{}
			err := node1.GetRegistry().UnmarshalMessage(in.Msg, request)
			require.NoError(t, err)
			logger.Info().Any("received request", request).Msg("test DNSReadRequestMessage")
			require.Equal(t, request.Domain, dnsEntry.Domain)
			require.Equal(t, request.TTL, dnsEntry.TTL)
			hasDNSReadRequest = true
			break
		}
	}

	insSize := len(ins)

	require.True(t, hasDNSReadRequest)

	outs := node1.GetOuts()
	//Fliter out DNSReadReplyMessage, find the one that matches the domain, check if all fields are respected
	for _, out := range outs {
		if out.Msg.Type == "DNSReadReplyMessage" {
			reply := &types.DNSReadReplyMessage{}
			err := node1.GetRegistry().UnmarshalMessage(out.Msg, reply)
			require.NoError(t, err)
			logger.Info().Any("reply", reply).Msg("test DNSReadReplyMessage")
			require.Equal(t, reply.Domain, dnsEntry.Domain)
			require.Equal(t, reply.IPAddress, IPAddress)
			require.LessOrEqual(t, reply.Expiration, Expiration) //LessOrEqual because of imprecision error, but should virtually be the same
			require.Equal(t, reply.Owner, Owner)
			break
		}
	}

	outsSize := len(outs)

	time.Sleep(time.Second * 1)

	//Send a DNSUpdateRequestMessage
	newDomain := "example2.com"
	newExpiration := time.Now().Add(time.Hour * 2)
	newOwner := "owner2"
	newIPAddress := "200.200.200.200"

	nameUpdateTx := types.Transaction{
		ID:          "tx3",
		Type:        types.NameUpdate,
		Input:       types.UTXO{DomainName: dnsEntry.Domain, IP: IPAddress, Owner: Owner, Expiration: Expiration},    // Previous Input UTXO of the domain
		Output:      types.UTXO{DomainName: newDomain, IP: newIPAddress, Owner: newOwner, Expiration: newExpiration}, // New UTXO of the domain
		PlainDomain: "",
		Salt:        "",
		Fees:        1,
	}

	msg = types.TransactionMessage{
		Tx: nameUpdateTx,
	}
	transpStatusMsg, err = node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header = transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node2.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameFirstUpdate transaction from reader POV (should not work since he is not the owner)
	err = proposer.Send(node2.GetAddr(), packet, time.Second*1)

	//After Failed NameUpdate (cannot change Owner), check if the DNS entry is still the same

	//Wait 1 seconds, it should not be needed since we should right after the NameFirstUpdate
	time.Sleep(time.Second * 1)

	transpRequestMsg, err = node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header = transport.NewHeader(reader.GetAddress(), reader.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpRequestMsg,
	}

	// Send DNSReadRequestMessage
	err = reader.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	//Check if DNSReadReplyMessage was received
	ins = node1.GetIns()

	//Check if one of the message received is a DNSReadRequestMessage
	hasDNSReadRequest = false
	for indexIns, in := range ins {
		if indexIns < insSize {
			continue
		}
		if in.Msg.Type == "DNSReadRequestMessage" {
			request := &types.DNSReadRequestMessage{}
			err := node1.GetRegistry().UnmarshalMessage(in.Msg, request)
			require.NoError(t, err)
			logger.Info().Any("received request", request).Msg("test DNSReadRequestMessage")
			require.Equal(t, request.Domain, dnsEntry.Domain)
			require.Equal(t, request.TTL, dnsEntry.TTL)
			hasDNSReadRequest = true
			break
		}
	}

	require.True(t, hasDNSReadRequest)

	outs = node1.GetOuts()
	//Fliter out DNSReadReplyMessage, find the one that matches the domain, check if all fields are respected
	for indexOuts, out := range outs {
		if indexOuts < outsSize {
			continue
		}
		if out.Msg.Type == "DNSReadReplyMessage" {
			reply := &types.DNSReadReplyMessage{}
			err := node1.GetRegistry().UnmarshalMessage(out.Msg, reply)
			require.NoError(t, err)
			logger.Info().Any("reply", reply).Msg("test DNSReadReplyMessage")
			require.Equal(t, reply.Domain, dnsEntry.Domain)
			require.Equal(t, reply.IPAddress, IPAddress)
			require.LessOrEqual(t, reply.Expiration, Expiration) //LessOrEqual because of imprecision error, but should virtually be the same
			require.Equal(t, reply.Owner, Owner)
			break
		}
	}

	time.Sleep(time.Second * 1)

	//Send a corect DNSUpdateRequestMessage
	newDomain = "example.com" //Keep the same domain as before
	//Change other fields
	newExpiration = time.Now().Add(time.Hour * 3)
	newOwner = "owner2"
	newIPAddress = "200.200.200.200"

	nameUpdateTx = types.Transaction{
		ID:          "tx3",
		Type:        types.NameUpdate,
		Input:       types.UTXO{DomainName: dnsEntry.Domain, IP: IPAddress, Owner: Owner, Expiration: Expiration},    // Previous Input UTXO of the domain
		Output:      types.UTXO{DomainName: newDomain, IP: newIPAddress, Owner: newOwner, Expiration: newExpiration}, // New UTXO of the domain
		PlainDomain: "",
		Salt:        "",
		Fees:        1,
	}

	msg = types.TransactionMessage{
		Tx: nameUpdateTx,
	}
	transpStatusMsg, err = node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header = transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node2.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameUpdate transaction from reader POV (should  work now)

	//TODO: check for if the sender is the actual OWNER !!!!!!!
	err = proposer.Send(node2.GetAddr(), packet, time.Second*1)

	insSize = len(node1.GetIns()) //get the size of ins before the message

	outsSize = len(node1.GetOuts()) //get the size of outs before the message

	//Wait 1 seconds, it should not be needed since we should right after the NameFirstUpdate
	time.Sleep(time.Second * 1)

	transpRequestMsg, err = node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header = transport.NewHeader(reader.GetAddress(), reader.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpRequestMsg,
	}

	// Send DNSReadRequestMessage
	err = reader.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	//Check if DNSReadReplyMessage was received
	ins = node1.GetIns()

	//Check if one of the message received is a DNSReadRequestMessage
	hasDNSReadRequest = false
	for indexIns, in := range ins {
		//Only target new messages after the previous Ins
		if indexIns < insSize {
			continue
		}
		if in.Msg.Type == "DNSReadRequestMessage" {
			request := &types.DNSReadRequestMessage{}
			err := node1.GetRegistry().UnmarshalMessage(in.Msg, request)
			require.NoError(t, err)
			logger.Info().Any("received request", request).Msg("test DNSReadRequestMessage")
			require.Equal(t, request.Domain, dnsEntry.Domain)
			require.Equal(t, request.TTL, dnsEntry.TTL)
			hasDNSReadRequest = true
			break
		}
	}

	require.True(t, hasDNSReadRequest)

	outs = node1.GetOuts()
	//Fliter out DNSReadReplyMessage, find the one that matches the domain, check if all fields are respected
	for indexOuts, out := range outs {
		//Only target new messages after the previous Outs
		if indexOuts < outsSize {
			continue
		}
		if out.Msg.Type == "DNSReadReplyMessage" {
			reply := &types.DNSReadReplyMessage{}
			err := node1.GetRegistry().UnmarshalMessage(out.Msg, reply)
			require.NoError(t, err)
			logger.Info().Any("reply", reply).Msg("test DNSReadReplyMessage")
			require.Equal(t, reply.Domain, newDomain)
			require.Equal(t, reply.IPAddress, newIPAddress)
			require.LessOrEqual(t, reply.Expiration, newExpiration) //LessOrEqual because of imprecision error, but should virtually be the same
			require.Equal(t, reply.Owner, newOwner)
			break
		}
	}

}
