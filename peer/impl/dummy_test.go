package impl

import (
	"errors"
	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"go.dedis.ch/cs438/transport"
	"sort"
	"sync"
	"testing"
	"time"
)

// TODO This file serves as a test and example for the coverage tool and should be deleted on final submission
// The below test is simply copied from the existing HW0_test.go
// 0-3
//
// Test multiple send/recv concurrently
func Test_HW0_Network_Multiple(t *testing.T) {
	sendingN1 := 10
	sendingN2 := 10

	net1 := udpFac()
	net2 := udpFac()

	sock1, err := net1.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)
	defer sock1.Close()

	sock2, err := net2.CreateSocket("127.0.0.1:0")
	require.NoError(t, err)
	defer sock2.Close()

	n1Addr := sock1.GetAddress()
	n2Addr := sock2.GetAddress()

	n1Received := []transport.Packet{}
	n2Received := []transport.Packet{}

	n1Sent := []transport.Packet{}
	n2Sent := []transport.Packet{}

	stop := make(chan struct{})

	rcvWG := sync.WaitGroup{}
	rcvWG.Add(2)

	// Receiving loop for n1
	go func() {
		defer rcvWG.Done()

		for {
			select {
			case <-stop:
				return
			default:
				pkt, err := sock1.Recv(time.Millisecond * 10)
				if errors.Is(err, transport.TimeoutError(0)) {
					continue
				}

				require.NoError(t, err)
				n1Received = append(n1Received, pkt)
			}
		}
	}()

	// Receiving loop for n2
	go func() {
		defer rcvWG.Done()

		for {
			select {
			case <-stop:
				return
			default:
				pkt, err := sock2.Recv(time.Millisecond * 10)
				if errors.Is(err, transport.TimeoutError(0)) {
					continue
				}

				require.NoError(t, err)
				n2Received = append(n2Received, pkt)
			}
		}
	}()

	sendWG := sync.WaitGroup{}
	sendWG.Add(2)

	// Sending loop for n1
	go func() {
		defer sendWG.Done()

		for i := 0; i < sendingN1; i++ {
			pkt := z.GetRandomPkt(t)
			n1Sent = append(n1Sent, pkt)
			err := sock1.Send(n2Addr, pkt, 0)
			require.NoError(t, err)
		}
	}()

	// Sending loop for n2
	go func() {
		defer sendWG.Done()

		for i := 0; i < sendingN2; i++ {
			pkt := z.GetRandomPkt(t)
			n2Sent = append(n2Sent, pkt)
			err := sock2.Send(n1Addr, pkt, 0)
			require.NoError(t, err)
		}
	}()

	// wait for both nodes to send their packets
	sendWG.Wait()

	time.Sleep(time.Second * 1)

	close(stop)

	// wait for listening node to finish
	rcvWG.Wait()

	sort.Sort(transport.ByPacketID(n1Received))
	sort.Sort(transport.ByPacketID(n2Received))
	sort.Sort(transport.ByPacketID(n1Sent))
	sort.Sort(transport.ByPacketID(n2Sent))

	require.Equal(t, n1Sent, n2Received)
	require.Equal(t, n2Sent, n1Received)
}
