package udp

import (
	"errors"
	"math"
	"net"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.dedis.ch/cs438/transport"
	"golang.org/x/xerrors"
)

var (
	defaultLevel = zerolog.InfoLevel
	logout       = zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
	logger zerolog.Logger
)

func init() {
	if os.Getenv("GLOG") == "no" {
		defaultLevel = zerolog.Disabled
	}
	logger = zerolog.New(logout).
		Level(defaultLevel).
		With().
		Timestamp().
		Logger()
}

// It is advised to define a constant (max) size for all relevant byte buffers, e.g:
const bufSize = 65000

// NewUDP returns a new udp transport implementation.
func NewUDP() transport.Transport {
	return &UDP{}
}

// UDP implements a transport layer using UDP
//
// - implements transport.Transport
type UDP struct {
}

// CreateSocket implements transport.Transport
func (n *UDP) CreateSocket(address string) (transport.ClosableSocket, error) {
	// Get UDP address
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		logger.Error().
			Str("method", "CreateSocket").
			Str("address", address).
			Err(err).
			Msg("Failed to resolve UDP address")
		return nil, err
	}
	// Start listening
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		logger.Error().
			Str("method", "CreateSocket").
			Str("address", address).
			Err(err).
			Msg("Failed to listen on UDP address")
		return nil, err
	}

	// Create socket
	s := &Socket{
		conn:    conn,
		address: conn.LocalAddr().String(),
		ins:     []transport.Packet{},
		outs:    []transport.Packet{},
	}

	// log := s.getLogger()
	// log.Info().
	// 	Str("method", "CreateSocket").
	// 	Msg("Socket created successfully")

	return s, nil
}

// Socket implements a network socket using UDP.
//
// - implements transport.Socket
// - implements transport.ClosableSocket
type Socket struct {
	conn    *net.UDPConn
	address string

	// Arrays to store inputs and outputs of the socket
	ins  []transport.Packet
	outs []transport.Packet

	// Locks to handle accessing the arrays
	insLock  sync.Mutex
	outsLock sync.Mutex

	// Flag and lock for closing socket
	closed     bool
	closeLock  sync.Mutex
}

// Helper method to get the socket-specific logger
func (s *Socket) getLogger() zerolog.Logger {
	return logger.With().Str("socket_address", s.address).Logger()
}

// Close implements transport.Socket. It returns an error if already closed.
func (s *Socket) Close() error {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()

	 log := s.getLogger()

	if s.closed {
		log.Warn().
			Str("method", "Close").
			Msg("Socket already closed")
		return xerrors.Errorf("socket already closed")
	}
	s.closed = true

	if err := s.conn.Close(); err != nil {
		log.Error().
			Str("method", "Close").
			Err(err).
			Msg("Failed to close socket")
		return err
	}

	log.Info().
		Str("method", "Close").
		Msg("Socket closed successfully")

	return nil
}

// Send implements transport.Socket
func (s *Socket) Send(dest string, pkt transport.Packet, timeout time.Duration) error {
	log := s.getLogger()

	// Marshal the packet
	data, err := pkt.Marshal()
	if err != nil {
		log.Error().
			Str("method", "Send").
			Str("destination", dest).
			Err(err).
			Msg("Failed to marshal packet")
		return err
	}

	// Get UDP address of dest
	udpAddr, err := net.ResolveUDPAddr("udp", dest)
	if err != nil {
		log.Error().
			Str("method", "Send").
			Str("destination", dest).
			Err(err).
			Msg("Failed to resolve UDP address")
		return xerrors.Errorf("failed to resolve udp address %s: %w", dest, err)
	}

	// Set timeout
	if timeout == 0 {
		timeout = math.MaxInt64
	}

	// Set deadline for sending the message
	err = s.conn.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		log.Error().
			Str("method", "Send").
			Str("destination", dest).
			Err(err).
			Msg("Failed to set write deadline")
		return xerrors.Errorf("failed to set write deadline: %w", err)
	}

	// Actually send the message
	_, err = s.conn.WriteToUDP(data, udpAddr)
	if err != nil {
		log.Error().
			Str("method", "Send").
			Str("destination", dest).
			Err(err).
			Msg("Failed to send data")
		return xerrors.Errorf("failed to send data: %w", err)
	}

	// Add the packet to the output array of the socket
	s.outsLock.Lock()
	s.outs = append(s.outs, pkt)
	s.outsLock.Unlock()

	log.Info().
		Str("method", "Send").
		Str("destination", dest).
		Msg("Packet sent successfully")

	return nil
}


// Recv implements transport.Socket. It blocks until a packet is received, or
// the timeout is reached. In the case the timeout is reached, return a
// TimeoutErr.
func (s *Socket) Recv(timeout time.Duration) (transport.Packet, error) {
	log := s.getLogger()

	// Set read deadline for receiving the message
	if timeout > 0 {
		if err := s.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			log.Error().
				Str("method", "Recv").
				Err(err).
				Msg("Failed to set read deadline")
			 return transport.Packet{}, xerrors.Errorf("failed to set read deadline: %w", err)
		}
	}

	// Prepare buffer to receive data
	buf := make([]byte, bufSize)

	// Read from UDP connection
	n, _, err := s.conn.ReadFromUDP(buf)
	if err != nil {
		if isTimeoutError(err) {
			log.Warn().
				Str("method", "Recv").
				Dur("timeout", timeout).
				Msg("Receive timeout reached")
			return transport.Packet{}, transport.TimeoutError(timeout)
		}
		log.Error().
			Str("method", "Recv").
			Err(err).
			Msg("Failed to read from UDP")
		return transport.Packet{}, xerrors.Errorf("failed to read from UDP: %w", err)
	}

	// Deserialize the packet
	var pkt transport.Packet
	err = pkt.Unmarshal(buf[:n])
	if err != nil {
		log.Error().
			Str("method", "Recv").
			Err(err).
			Msg("Failed to unmarshal packet")
		return transport.Packet{}, xerrors.Errorf("failed to unmarshal packet: %w", err)
	}

	// Record the received packet
	s.insLock.Lock()
	s.ins = append(s.ins, pkt)
	s.insLock.Unlock()


	return pkt, nil
}

// GetAddress implements transport.Socket. It returns the address assigned. Can
// be useful in the case one provided a :0 address, which makes the system use a
// random free port.
func (s *Socket) GetAddress() string {
	return s.address
}

// Deep copies
// GetIns implements transport.Socket
func (s *Socket) GetIns() []transport.Packet {
	s.insLock.Lock()
	defer s.insLock.Unlock()

	cp := make([]transport.Packet, len(s.ins))
	copy(cp, s.ins)

	return cp
}

// GetOuts implements transport.Socket
func (s *Socket) GetOuts() []transport.Packet {
	s.outsLock.Lock()
	defer s.outsLock.Unlock()

	cp := make([]transport.Packet, len(s.outs))
	copy(cp, s.outs)

	return cp
}

// Auxiliary function to check if an error is a timeout error
func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
