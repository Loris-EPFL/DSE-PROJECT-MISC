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

func Test_HandleConcurrentDNSRegistration(t *testing.T) {
	transp := channel.NewTransport()

	// Create nodes with a higher difficulty to ensure longer mining time
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithPowDiff(0x1fffffff))
	defer node1.Stop()

	//node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.2:0", z.WithPowDiff(0x1fff1111))
	//defer node2.Stop()

	proposer1, err := z.NewSenderSocket(transp, "127.0.0.3:0")
	require.NoError(t, err)

	proposer2, err := z.NewSenderSocket(transp, "127.0.0.4:0")
	require.NoError(t, err)

	//node1.AddPeer(node2.GetAddr())
	//node2.AddPeer(node1.GetAddr())

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

	header := transport.NewHeader(proposer1.GetAddress(), proposer1.GetAddress(), node1.GetAddr())

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameNew transaction
	err = proposer1.Send(node1.GetAddr(), packet, time.Second*1)
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

	header = transport.NewHeader(proposer1.GetAddress(), proposer1.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameFirstUpdate transaction
	err = proposer1.Send(node1.GetAddr(), packet, time.Second*1)

	require.NoError(t, err)

	// Wait a short time to ensure mining has started
	//time.Sleep(time.Millisecond * 100)

	// Second DNS registration while first one is being mined
	domain2 := "example2.com"
	salt2 := "random_salt2"
	IPAddress2 := "192.168.1.2"
	owner2 := "owner2"
	Expiration2 := time.Now().Add(time.Hour)
	hash2 := sha256.New()
	hash2.Write([]byte(salt2 + domain2))
	saltedHash2 := hex.EncodeToString(hash2.Sum(nil))

	nameNewTx2 := types.Transaction{
		ID:           "tx3",
		Type:         types.NameNew,
		Input:        types.UTXO{},
		Output:       types.UTXO{DomainName: saltedHash2, IP: IPAddress2, Owner: owner2, Expiration: Expiration2},
		HashedDomain: saltedHash2,
		Fees:         1,
	}

	msg2 := types.TransactionMessage{Tx: nameNewTx2}
	transpMsg2, err := node1.GetRegistry().MarshalMessage(&msg2)
	require.NoError(t, err)

	// Send second NameNew transaction while first one is being mined
	err = proposer2.Send(node1.GetAddr(), transport.Packet{
		Header: &transport.Header{Source: proposer2.GetAddress(), RelayedBy: proposer2.GetAddress(), Destination: node1.GetAddr()},
		Msg:    &transpMsg2,
	}, time.Second)
	require.NoError(t, err)

	//Wait 3 seconds, it should instead be wait for about 3 blocks to be mined
	time.Sleep(time.Second * 5)

	// Create NameFirstUpdate transaction
	nameFirstUpdateTx2 := types.Transaction{
		ID:          "tx4",
		Type:        types.NameFirstUpdate,
		Input:       nameNewTx2.Output, // Dummy input
		Output:      types.UTXO{DomainName: domain2, IP: IPAddress2, Owner: owner2, Expiration: Expiration2},
		PlainDomain: domain2,
		Salt:        "random_salt",
		Fees:        1,
	}

	msg = types.TransactionMessage{
		Tx: nameFirstUpdateTx2,
	}
	transpStatusMsg, err = node1.GetRegistry().MarshalMessage(&msg)
	require.NoError(t, err)

	header = transport.NewHeader(proposer1.GetAddress(), proposer1.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send second NameFirstUpdate transaction
	err = proposer1.Send(node1.GetAddr(), packet, time.Second*1)

	require.NoError(t, err)

	// Wait for mining to complete
	time.Sleep(time.Second * 5)

	node1outs := node1.GetOuts()
	//node2outs := node2.GetOuts()

	for _, out := range node1outs {
		logger.Info().Msgf("Node1 out: %s", out)
	}

	/*
		for _, out := range node2outs {
			logger.Info().Msgf("Node2 out: %s", out)
		}
	*/

	node1ins := node1.GetIns()
	//node2ins := node2.GetIns()

	for _, in := range node1ins {
		logger.Info().Msgf("Node1 in: %s", in)
	}

	/*
		for _, in := range node2ins {
			logger.Info().Msgf("Node2 in: %s", in)
		}
	*/

	// Verify both transactions are in the blockchain
	// This part depends on your implementation's way to query the blockchain
	// You might need to add methods to your node implementation to check this

	// Example verification (adjust according to your implementation):
	// require.True(t, node1.HasTransaction(nameNewTx1.ID))
	// require.True(t, node1.HasTransaction(nameNewTx2.ID))
}
