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

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithPowBits(3))
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.2:0", z.WithPowBits(3))
	defer node2.Stop()

	proposer, err := z.NewSenderSocket(transp, "127.0.0.3:0")
	require.NoError(t, err)

	node1.AddPeer(node2.GetAddr())
	node2.AddPeer(node1.GetAddr())

	// Register a DNS entry
	dnsEntry := types.DNSReadRequestMessage{
		Domain: "example.com",
		TTL:    time.Second,
	}

	// Create NameNew transaction
	hash := sha256.New()
	hash.Write([]byte("random_salt" + dnsEntry.Domain))
	saltedHash := hex.EncodeToString(hash.Sum(nil))

	nameNewTx := types.Transaction{
		ID:           "tx1",
		Type:         types.NameNew,
		Input:        types.UTXO{}, // Dummy input
		Output:       types.UTXO{DomainName: saltedHash, IP: "192.168.1.1", Owner: "owner1", Expiration: time.Now().Add(time.Hour)},
		HashedDomain: saltedHash,
		Fees:         1,
	}

	msg := types.TransactionMessage{
		Tx: nameNewTx,
	}
	transpStatusMsg, err := node1.GetRegistry().MarshalMessage(&msg)

	header := transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node1.GetAddr())

	packet := transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameNew transaction
	err = proposer.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 3)

	// Create NameFirstUpdate transaction
	nameFirstUpdateTx := types.Transaction{
		ID:          "tx2",
		Type:        types.NameFirstUpdate,
		Input:       nameNewTx.Output, // Dummy input
		Output:      types.UTXO{ DomainName: dnsEntry.Domain, IP: "192.168.1.1", Owner: "owner1", Expiration: time.Now().Add(time.Hour)},
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

	// Send
	err = proposer.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 3)

	transpRequestMsg, err := node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header = transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpRequestMsg,
	}

	// Send DNSReadRequestMessage
	err = proposer.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 3)

	//Check if DNSReadReplyMessage was received
	ins := node1.GetIns()
	//Received 2 acks and 2 transactions
	require.Len(t, ins, 5)

	pkt := ins[4]
	require.Equal(t, "DNSReadRequestMessage", pkt.Msg.Type)

	// reply := types.DNSReadReplyMessage{}
	// err = json.Unmarshal(pkt.Msg.Payload, &reply)
	// require.NoError(t, err)

	// require.Equal(t, dnsEntry.Domain, reply.Domain)
	// require.Equal(t, "192.168.1.1", reply.IPAddress)
	// require.Greater(t, reply.TTL, time.Duration(0))
}
