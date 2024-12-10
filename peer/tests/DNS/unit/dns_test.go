package unit

import (
	"os"
	"time"

	"github.com/rs/zerolog"
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

/*

// Test handleDNSReadMessage
func Test_HandleDNSReadMessage(t *testing.T) {
	transp := channel.NewTransport()

	node := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node.Stop()

	// Register a DNS entry
	dnsEntry := peer.DNSEntry{
		Domain:     "example.com",
		IPAddress:  "192.168.1.1",
		Expiration: time.Now().Add(time.Hour),
		Owner:      "0x" + node.GetAddr(),
	}

	// Create DNSRegisterMessageNew
	hash := sha256.New()
	hash.Write([]byte("random_salt" + dnsEntry.Domain))
	saltedHash := hex.EncodeToString(hash.Sum(nil))

	dnsRegisterMsgNew := types.DNSRegisterMessageNew{
		SaltedHash: saltedHash,
		Expiration: dnsEntry.Expiration,
		Owner:      dnsEntry.Owner,
		Fee:        1,
	}
	data, err := json.Marshal(&dnsRegisterMsgNew)
	require.NoError(t, err)

	msg := transport.Message{
		Type:    dnsRegisterMsgNew.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessageNew
	err = node.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Create DNSRegisterMessageFirstUpdate
	dnsRegisterMsgFirstUpdate := types.DNSRegisterMessageFirstUpdate{
		Domain:     dnsEntry.Domain,
		IPAddress:  dnsEntry.IPAddress,
		Expiration: dnsEntry.Expiration,
		Owner:      dnsEntry.Owner,
		Salt:       "random_salt",
		Fee:        1,
	}
	data, err = json.Marshal(&dnsRegisterMsgFirstUpdate)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsRegisterMsgFirstUpdate.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessageFirstUpdate
	err = node.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Create DNSReadRequestMessage
	dnsReadMsg := types.DNSReadRequestMessage{
		Domain: dnsEntry.Domain,
		TTL:    time.Second,
	}
	data, err = json.Marshal(&dnsReadMsg)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsReadMsg.Name(),
		Payload: data,
	}

	// Send DNSReadRequestMessage
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
	dnsEntry := peer.DNSEntry{
		Domain:     "example.com",
		IPAddress:  "192.168.1.1",
		Expiration: time.Now().Add(time.Hour),
		Owner:      "0x" + node1.GetAddr(),
	}

	// Create DNSRegisterMessageNew
	hash := sha256.New()
	hash.Write([]byte("random_salt" + dnsEntry.Domain))
	saltedHash := hex.EncodeToString(hash.Sum(nil))

	dnsRegisterMsgNew := types.DNSRegisterMessageNew{
		SaltedHash: saltedHash,
		Expiration: dnsEntry.Expiration,
		Owner:      dnsEntry.Owner,
		Fee:        1,
	}
	data, err := json.Marshal(&dnsRegisterMsgNew)
	require.NoError(t, err)

	logger.Warn().Any("domain ", dnsEntry.Domain).Msg("Starting DNSRenewalMessage test")

	msg := transport.Message{
		Type:    dnsRegisterMsgNew.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessageNew
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Create DNSRegisterMessageFirstUpdate
	dnsRegisterMsgFirstUpdate := types.DNSRegisterMessageFirstUpdate{
		Domain:     dnsEntry.Domain,
		IPAddress:  dnsEntry.IPAddress,
		Expiration: dnsEntry.Expiration,
		Owner:      dnsEntry.Owner,
		Salt:       "random_salt",
		Fee:        1,
	}
	data, err = json.Marshal(&dnsRegisterMsgFirstUpdate)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsRegisterMsgFirstUpdate.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessageFirstUpdate
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Create DNSReadRequestMessage
	dnsReadMsg := types.DNSReadRequestMessage{
		Domain: dnsEntry.Domain,
		TTL:    time.Second,
	}
	data, err = json.Marshal(&dnsReadMsg)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsReadMsg.Name(),
		Payload: data,
	}

	// Send DNSReadRequestMessage
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if DNS entry was renewed

	entry := node1.Peer.GetDNSStoreEntry(dnsEntry.Domain)
	logger.Warn().Any("dnsEntry", entry).Msg("Checking if DNS entry was set")

	// Create DNSRenewalMessage
	newExpiration := time.Now().Add(2 * time.Hour)
	dnsRenewalMsg := types.DNSUpdateMessage{
		Domain:     dnsEntry.Domain,
		Expiration: newExpiration,
		Owner:      dnsEntry.Owner,
		Fee:        1,
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

	// Try to renew the DNS entry with a different owner
	dnsRenewalMsgDifferentOwner := types.DNSUpdateMessage{
		Domain:     dnsEntry.Domain,
		Expiration: newExpiration,
		Owner:      "owner2",
		Fee:        1,
	}
	data, err = json.Marshal(&dnsRenewalMsgDifferentOwner)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsRenewalMsgDifferentOwner.Name(),
		Payload: data,
	}

	// Send DNSRenewalMessage with different owner
	err = node1.Broadcast(msg)
	require.Error(t, err, "Unauthorized renewal attempt by owner2 for domain example.com")
}

// Test handleDNSRegisterMessage
func Test_HandleDNSRegisterMessage(t *testing.T) {
	transp := channel.NewTransport()

	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:0")
	defer node1.Stop()

	// Create DNSRegisterMessageNew
	hash := sha256.New()
	hash.Write([]byte("random_salt" + "example.com"))
	saltedHash := hex.EncodeToString(hash.Sum(nil))

	dnsRegisterMsgNew := types.DNSRegisterMessageNew{
		SaltedHash: saltedHash,
		Expiration: time.Now().Add(time.Hour),
		Owner:      "0x" + node1.GetAddr(),
		Fee:        1,
	}
	data, err := json.Marshal(&dnsRegisterMsgNew)
	require.NoError(t, err)

	msg := transport.Message{
		Type:    dnsRegisterMsgNew.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessageNew
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Create DNSRegisterMessageFirstUpdate
	dnsRegisterMsgFirstUpdate := types.DNSRegisterMessageFirstUpdate{
		Domain:     "example.com",
		IPAddress:  "192.168.1.1",
		Expiration: time.Now().Add(time.Hour),
		Owner:      "0x" + node1.GetAddr(),
		Salt:       "random_salt",
		Fee:        1,
	}
	data, err = json.Marshal(&dnsRegisterMsgFirstUpdate)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsRegisterMsgFirstUpdate.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessageFirstUpdate
	err = node1.Broadcast(msg)
	require.NoError(t, err)

	time.Sleep(time.Second * 1)

	// Check if DNS entry was registered
	registeredEntry := node1.Peer.GetDNSStoreEntry(dnsRegisterMsgFirstUpdate.Domain)
	require.Equal(t, dnsRegisterMsgFirstUpdate.Domain, registeredEntry.Domain)
	require.Equal(t, dnsRegisterMsgFirstUpdate.IPAddress, registeredEntry.IPAddress)
	require.WithinDuration(t, dnsRegisterMsgFirstUpdate.Expiration, registeredEntry.Expiration, time.Second)

	// Try to register the same DNS entry with a different owner
	dnsRegisterMsgDifferentOwner := types.DNSRegisterMessageFirstUpdate{
		Domain:     "example.com",
		IPAddress:  "192.168.1.2",
		Expiration: time.Now().Add(time.Hour),
		Owner:      "owner2",
		Salt:       "random_salt",
		Fee:        1,
	}
	data, err = json.Marshal(&dnsRegisterMsgDifferentOwner)
	require.NoError(t, err)

	msg = transport.Message{
		Type:    dnsRegisterMsgDifferentOwner.Name(),
		Payload: data,
	}

	// Send DNSRegisterMessageFirstUpdate with different owner
	err = node1.Broadcast(msg)
	require.Error(t, err, "Unauthorized registration attempt by owner2 for domain example.com")
}
*/
