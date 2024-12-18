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

// Test handleDNSReadMessage
func Test_HandleFees(t *testing.T) {
	transp := channel.NewTransport()

	// Create 2 nodes and a reader (sender type since he "propose" a readDNSRequest)
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0", z.WithPowDiff(difficulty))
	defer node1.Stop()

	proposer, err := z.NewSenderSocket(transp, "127.0.0.3:0")
	require.NoError(t, err)

	reader, err := z.NewSenderSocket(transp, "127.0.0.4:0")
	require.NoError(t, err)

	// Register a DNS entry
	dnsEntry := types.DNSReadRequestMessage{
		Domain: "example.com",
		TTL:    time.Second,
	}

	// Fields for the DNS entry inside the block
	IPAddress := "192.168.1.1"
	Expiration := time.Now().Add(time.Hour)
	Owner := "owner1"
	balanceNode1, ok1 := nct.Balances.Get(node1.GetAddr())

	logger.Info().Any("node1 balance", balanceNode1).Msg("initial balance")

	nct.Balances.Add("owner1", 10) // Add initial balance for the owner

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

	// Wait 3 seconds, it should instead be wait for about 3 blocks to be mined
	time.Sleep(time.Second * 5)

	// Check balances after mining
	balanceNode1, ok1 = nct.Balances.Get(node1.GetAddr())
	logger.Info().Any("node1 balance", balanceNode1).Msg("balance after mining")
	require.True(t, ok1)

	ownerBalance, ok := nct.Balances.Get(Owner)
	require.True(t, ok)

	require.Equal(t, uint(30), balanceNode1) // 30 because 10 initial balance + 20 from the block mined
	require.Equal(t, uint(9), ownerBalance)  // 9 because 10 initial balance - 1 fee

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

	header = transport.NewHeader(proposer.GetAddress(), proposer.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpStatusMsg,
	}

	// Send NameFirstUpdate transaction
	err = proposer.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	// Wait 1 second, it should not be needed since we should right after the NameFirstUpdate
	time.Sleep(time.Second * 1)

	// Check balances after mining
	balanceNode1, ok1 = nct.Balances.Get(node1.GetAddr())
	logger.Info().Any("node1 balance", balanceNode1).Msg("balance after mining")
	require.True(t, ok1)

	ownerBalance, ok = nct.Balances.Get(Owner)
	require.True(t, ok)

	require.Equal(t, uint(50), balanceNode1) // because 10 initial balance + 20 from the  1st block mined
	require.Equal(t, uint(8), ownerBalance)  // 8 because 10 initial balance - 1 first update fee - 1 name new fee

	transpRequestMsg, err := node1.GetRegistry().MarshalMessage(&dnsEntry)
	require.NoError(t, err)

	header = transport.NewHeader(reader.GetAddress(), reader.GetAddress(), node1.GetAddr())

	packet = transport.Packet{
		Header: &header,
		Msg:    &transpRequestMsg,
	}

	//Get balance of Reader (should be 0)
	readerBalance, ok := nct.Balances.Get(reader.GetAddress())
	require.True(t, !ok) // Reader should not have balance
	logger.Info().Any("reader balance", readerBalance).Msg("reader balance")

	// Send DNSReadRequestMessage
	err = reader.Send(node1.GetAddr(), packet, time.Second*1)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if DNSReadReplyMessage was received
	ins := node1.GetIns()
	// logger.Info().Any("ins", ins).Msg("Received messages")

	// Check if one of the message received is a DNSReadRequestMessage
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
	// Filter out DNSReadReplyMessage, find the one that matches the domain, check if all fields are respected
	for _, out := range outs {
		if out.Msg.Type == "DNSReadReplyMessage" {
			reply := &types.DNSReadReplyMessage{}
			err := node1.GetRegistry().UnmarshalMessage(out.Msg, reply)
			require.NoError(t, err)
			logger.Info().Any("reply", reply).Msg("test DNSReadReplyMessage")
			require.Equal(t, reply.Domain, dnsEntry.Domain)
			require.Equal(t, reply.IPAddress, IPAddress)
			require.LessOrEqual(t, reply.Expiration, Expiration) // LessOrEqual because of imprecision error, but should virtually be the same
			require.Equal(t, reply.Owner, Owner)
			break
		}
	}

	//Get balance of Reader (should still be 0 after read)
	_, ok = nct.Balances.Get(reader.GetAddress())
	require.True(t, !ok) // Reader should not have balance

}
