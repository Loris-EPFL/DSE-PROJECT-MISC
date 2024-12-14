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

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.2:0", z.WithPowDiff(0x1fffffff))
	defer node2.Stop()

	proposer1, err := z.NewSenderSocket(transp, "127.0.0.3:0")
	require.NoError(t, err)

	proposer2, err := z.NewSenderSocket(transp, "127.0.0.4:0")
	require.NoError(t, err)

	node1.AddPeer(node2.GetAddr())
	node2.AddPeer(node1.GetAddr())

	// First DNS registration
	domain1 := "example1.com"
	salt1 := "random_salt1"
	hash1 := sha256.New()
	hash1.Write([]byte(salt1 + domain1))
	saltedHash1 := hex.EncodeToString(hash1.Sum(nil))

	nameNewTx1 := types.Transaction{
		ID:           "tx1",
		Type:         types.NameNew,
		Input:        types.UTXO{},
		Output:       types.UTXO{DomainName: saltedHash1, IP: "192.168.1.1", Owner: "owner1", Expiration: time.Now().Add(time.Hour)},
		HashedDomain: saltedHash1,
		Fees:         1,
	}

	msg1 := types.TransactionMessage{Tx: nameNewTx1}
	transpMsg1, err := node1.GetRegistry().MarshalMessage(&msg1)
	require.NoError(t, err)

	// Send first NameNew transaction
	err = proposer1.Send(node1.GetAddr(), transport.Packet{
		Header: &transport.Header{Source: proposer1.GetAddress(), RelayedBy: proposer1.GetAddress(), Destination: node1.GetAddr()},
		Msg:    &transpMsg1,
	}, time.Second)
	require.NoError(t, err)

	// Wait a short time to ensure mining has started
	time.Sleep(time.Millisecond * 100)

	// Second DNS registration while first one is being mined
	domain2 := "example2.com"
	salt2 := "random_salt2"
	hash2 := sha256.New()
	hash2.Write([]byte(salt2 + domain2))
	saltedHash2 := hex.EncodeToString(hash2.Sum(nil))

	nameNewTx2 := types.Transaction{
		ID:           "tx3",
		Type:         types.NameNew,
		Input:        types.UTXO{},
		Output:       types.UTXO{DomainName: saltedHash2, IP: "192.168.1.2", Owner: "owner2", Expiration: time.Now().Add(time.Hour)},
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

	// Wait for mining to complete
	time.Sleep(time.Second * 5)

	// Verify both transactions are in the blockchain
	// This part depends on your implementation's way to query the blockchain
	// You might need to add methods to your node implementation to check this

	// Example verification (adjust according to your implementation):
	// require.True(t, node1.HasTransaction(nameNewTx1.ID))
	// require.True(t, node1.HasTransaction(nameNewTx2.ID))
}
