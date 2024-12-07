package unit

import (
	"encoding/json"
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

// DNSEntry represents a DNS entry containing essential information
type DNSEntry struct {
	Domain     string
	IPAddress  string
	Expiration time.Time
}

// Test handleDNSReadMessage
func Test_HandleDNSReadMessage(t *testing.T) {
	transp := channel.NewTransport()

	node := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node.Stop()

	// Register a DNS entry
	dnsEntry := DNSEntry{
		Domain:     "example.com",
		IPAddress:  "192.168.1.1",
		Expiration: time.Now().Add(time.Hour),
	}

	// Create DNSRegisterMessage
	dnsRegisterMsg := types.DNSRegisterMessage{
		Domain:     dnsEntry.Domain,
		IPAddress:  dnsEntry.IPAddress,
		Expiration: dnsEntry.Expiration,
	}
	data, err := json.Marshal(&dnsRegisterMsg)
	require.NoError(t, err)

	msg := transport.Message{
		Type:    dnsRegisterMsg.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessage
	err = node.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Create DNSReadMessage
	dnsReadMsg := types.DNSReadMessage{
		Domain: dnsEntry.Domain,
		TTL:    time.Second,
	}
	data, err = json.Marshal(&dnsReadMsg)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsReadMsg.Name(),
		Payload: data,
	}

	// Send DNSReadMessage
	err = node.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if DNSReadReplyMessage was received
	ins := node.GetIns()
	require.Len(t, ins, 1)

	pkt := ins[0]
	require.Equal(t, "DNSReadReplyMessage", pkt.Msg.Type)

	reply := types.DNSReadReplyMessage{}
	err = json.Unmarshal(pkt.Msg.Payload, &reply)
	require.NoError(t, err)

	require.Equal(t, dnsEntry.Domain, reply.Domain)
	require.Equal(t, dnsEntry.IPAddress, reply.IPAddress)
	require.Greater(t, reply.TTL, time.Duration(0))
}

// Test handleDNSRenewalMessage
func Test_HandleDNSRenewalMessage(t *testing.T) {
	transp := channel.NewTransport()

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node1.Stop()

	node2 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node2.Stop()

	node1.AddPeer(node2.GetAddr())

	// Register a DNS entry
	dnsEntry := DNSEntry{
		Domain:     "example.com",
		IPAddress:  "192.168.1.1",
		Expiration: time.Now().Add(time.Hour),
	}

	// Create DNSRegisterMessage
	dnsRegisterMsg := types.DNSRegisterMessage{
		Domain:     dnsEntry.Domain,
		IPAddress:  dnsEntry.IPAddress,
		Expiration: dnsEntry.Expiration,
	}
	data, err := json.Marshal(&dnsRegisterMsg)
	require.NoError(t, err)

	logger.Warn().Any("domain ", dnsEntry.Domain).Msg("Starting DNSRenewalMessage test")

	msg := transport.Message{
		Type:    dnsRegisterMsg.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessage
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Create DNSReadMessage
	dnsReadMsg := types.DNSReadMessage{
		Domain: dnsEntry.Domain,
		TTL:    time.Second,
	}
	data, err = json.Marshal(&dnsReadMsg)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsReadMsg.Name(),
		Payload: data,
	}

	// Send DNSReadMessage
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if DNS entry was renewed

	entry := node1.Peer.GetDNSStoreEntry(dnsEntry.Domain)
	logger.Warn().Any("dnsEntry", entry).Msg("Checking if DNS entry was set")

	// Create DNSRenewalMessage
	newExpiration := time.Now().Add(2 * time.Hour)
	dnsRenewalMsg := types.DNSRenewalMessage{
		Domain:     dnsEntry.Domain,
		Expiration: newExpiration,
	}
	data, err = json.Marshal(&dnsRenewalMsg)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsRenewalMsg.Name(),
		Payload: data,
	}

	logger.Warn().Any("domain ", dnsEntry.Domain).Any("expiration", newExpiration).Msg("Sending DNSRenewalMessage")

	// Send DNSRenewalMessage
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if DNS entry was renewed
	updatedEntry := node1.Peer.GetDNSStoreEntry(dnsEntry.Domain)
	logger.Warn().Any("dnsEntry", updatedEntry).Any("expiration", updatedEntry.Expiration).Msg("Checking if DNS entry was renewed")

	require.WithinDuration(t, dnsRenewalMsg.Expiration, updatedEntry.Expiration, time.Second)
}

// Test handleDNSRegisterMessage
func Test_HandleDNSRegisterMessage(t *testing.T) {
	transp := channel.NewTransport()

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node1.Stop()

	// Create DNSRegisterMessage
	dnsRegisterMsg := types.DNSRegisterMessage{
		Domain:     "example.com",
		IPAddress:  "192.168.1.1",
		Expiration: time.Now().Add(time.Hour),
	}
	data, err := json.Marshal(&dnsRegisterMsg)
	require.NoError(t, err)

	msg := transport.Message{
		Type:    dnsRegisterMsg.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessage
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if DNS entry was registered
	registeredEntry := node1.Peer.GetDNSStoreEntry(dnsRegisterMsg.Domain)
	require.Equal(t, dnsRegisterMsg.Domain, registeredEntry.Domain)
	require.Equal(t, dnsRegisterMsg.IPAddress, registeredEntry.IPAddress)
	require.WithinDuration(t, dnsRegisterMsg.Expiration, registeredEntry.Expiration, time.Second)
}
