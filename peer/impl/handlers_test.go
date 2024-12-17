package impl

import (
	"crypto/sha256"
	"encoding/hex"
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

func Test_HandleExpiredDomain(t *testing.T) {
	transp := channel.NewTransport()

	// Create nodes with a higher difficulty to ensure longer mining time
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithPowDiff(difficulty))
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.2:0", z.WithPowDiff(difficulty))

	defer node2.Stop()

	//Create proposers to send the read request
	proposer1, err := z.NewSenderSocket(transp, "127.0.0.3:0")
	require.NoError(t, err)

	//Create proposers to send the read request
	proposer2, err := z.NewSenderSocket(transp, "127.0.0.4:0")
	require.NoError(t, err)

	node1.AddPeer(node2.GetAddr())
	node2.AddPeer(node1.GetAddr())

	// Register a DNS entry
	dnsEntry := types.DNSReadRequestMessage{
		Domain: "example.com",
		TTL:    time.Second,
	}

	// Fields for the DNS entry inside the block
	IPAddress := "192.168.1.1"
	Expiration := time.Now().Add(time.Second * 10) // Short expiration time for testing
	Owner := "owner1"

	// Create NameNew transaction
	hash := sha256.New()
	hash.Write([]byte("random_salt" + dnsEntry.Domain))
	saltedHash := hex.EncodeToString(hash.Sum(nil))

	// Create NameNew transaction
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
	// Marshal the message
	transpStatusMsg, err := node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header := transport.NewHeader(proposer1.GetAddress(), proposer1.GetAddress(), node1.GetAddr())

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameNew transaction
	err = proposer1.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	// Wait for the transaction to be mined
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

	// Marshal the message
	transpStatusMsg, err = node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header = transport.NewHeader(proposer1.GetAddress(), proposer1.GetAddress(), node2.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameFirstUpdate transaction
	err = proposer1.Send(node2.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	// Wait for the transaction to be mined
	time.Sleep(time.Second * 3)

	// Wait for the domain to expire
	time.Sleep(time.Second * 10)

	// Check if the domain can be accessed after expiration
	transpRequestMsg, err := node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header = transport.NewHeader(proposer1.GetAddress(), proposer1.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpRequestMsg,
	}

	// Send DNSReadRequestMessage
	err = proposer1.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if DNSReadReplyMessage was received
	ins := node1.GetIns()
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

	//Need to have received a DNSReadRequestMessage
	require.True(t, hasDNSReadRequest)

	outs := node1.GetOuts()
	for _, out := range outs {
		if out.Msg.Type == "DNSReadReplyMessage" {
			reply := &types.DNSReadReplyMessage{}
			err := node1.GetRegistry().UnmarshalMessage(out.Msg, reply)
			require.NoError(t, err)
			logger.Info().Any("reply", reply).Msg("test DNSReadReplyMessage")
			require.Equal(t, reply.Domain, dnsEntry.Domain)
			require.Equal(t, reply.IPAddress, "expired")         // Expired domain
			require.Equal(t, reply.Exists, false)                // Domain should not exist anymore
			require.LessOrEqual(t, reply.Expiration, Expiration) // LessOrEqual because of imprecision error, but should virtually be the same
			require.Equal(t, reply.Owner, Owner)
			break
		}
	}

	// Register the same domain by another owner after expiration
	newOwner := "owner2"
	newIPAddress := "192.168.1.2"
	newExpiration := time.Now().Add(time.Hour)

	// Create NameNew transaction
	hash = sha256.New()
	hash.Write([]byte("random_salt2" + dnsEntry.Domain))
	saltedHash = hex.EncodeToString(hash.Sum(nil))

	// Create NameNew transaction for the new owner
	nameNewTx2 := types.Transaction{
		ID:           "tx3",
		Type:         types.NameNew,
		Input:        types.UTXO{}, // Dummy input
		Output:       types.UTXO{DomainName: saltedHash, IP: newIPAddress, Owner: newOwner, Expiration: newExpiration},
		HashedDomain: saltedHash,
		Fees:         1,
	}

	msg2 := types.TransactionMessage{
		Tx: nameNewTx2,
	}
	transpStatusMsg2, err := node1.GetRegistry().MarshalMessage(&msg2)
	require.NoError(t, err)

	header2 := transport.NewHeader(proposer2.GetAddress(), proposer2.GetAddress(), node1.GetAddr())

	packet2 := transport.Packet{
		Header: &header2,
		Msg:    &transpStatusMsg2,
	}

	// Send NameNew transaction for the new owner
	err = proposer2.Send(node1.GetAddr(), packet2, time.Second*1)
	require.NoError(t, err)

	// Wait for the transaction to be mined
	time.Sleep(time.Second * 5)

	// Create NameFirstUpdate transaction
	nameFirstUpdateTx2 := types.Transaction{
		ID:          "tx4",
		Type:        types.NameFirstUpdate,
		Input:       nameNewTx2.Output, // Dummy input
		Output:      types.UTXO{DomainName: dnsEntry.Domain, IP: newIPAddress, Owner: newOwner, Expiration: newExpiration},
		PlainDomain: dnsEntry.Domain,
		Salt:        "random_salt2",
		Fees:        1,
	}

	msg = types.TransactionMessage{
		Tx: nameFirstUpdateTx2,
	}
	transpStatusMsg, err = node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header = transport.NewHeader(proposer1.GetAddress(), proposer1.GetAddress(), node2.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send second NameFirstUpdate transaction
	err = proposer1.Send(node2.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	// Check if the new owner can access the domain
	transpRequestMsg2, err := node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header2 = transport.NewHeader(proposer2.GetAddress(), proposer2.GetAddress(), node1.GetAddr())

	packet2 = transport.Packet{
		Header: &header2,
		Msg:    &transpRequestMsg2,
	}

	node1OutsSize := len(node1.GetOuts()) // Save the number of outputs for later sorting

	node1InsSize := len(node1.GetIns()) // Save the number of inputs for later sorting

	// Send DNSReadRequestMessage
	err = proposer2.Send(node1.GetAddr(), packet2, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if the domain can be accessed after we changed it
	transpRequestMsg, err = node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header = transport.NewHeader(proposer1.GetAddress(), proposer1.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpRequestMsg,
	}

	// Send DNSReadRequestMessage
	err = proposer1.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 2)

	// Check if DNSReadReplyMessage was received
	ins = node1.GetIns()
	hasDNSReadRequest = false
	for index, in := range ins {
		if index <= node1InsSize {
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

	//Need to have received a DNSReadRequestMessage
	require.True(t, hasDNSReadRequest)

	outs = node1.GetOuts()
	for index, out := range outs {
		if index <= node1OutsSize {
			continue
		}
		if out.Msg.Type == "DNSReadReplyMessage" {
			reply := &types.DNSReadReplyMessage{}
			err := node1.GetRegistry().UnmarshalMessage(out.Msg, reply)
			require.NoError(t, err)
			if reply.Owner == newOwner { // Check if the reply maps to the new owner
				logger.Info().Any("reply", reply).Msg("test DNSReadReplyMessage 2")
				require.Equal(t, reply.Domain, dnsEntry.Domain)         // Check if the domain is the same
				require.Equal(t, reply.IPAddress, newIPAddress)         // Check if the IPAddress is the same
				require.LessOrEqual(t, reply.Expiration, newExpiration) // LessOrEqual because of imprecision error, but should virtually be the same
				require.Equal(t, reply.Owner, newOwner)
				break
			}
		}
	}
}
