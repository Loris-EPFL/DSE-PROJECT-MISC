package types

import (
	"fmt"
	"time"
)

// DNSReadMessage represents a message to read a DNS entry.
type DNSReadRequestMessage struct {
	Domain string
	TTL    time.Duration
}

func (m DNSReadRequestMessage) NewEmpty() Message {
	return &DNSReadRequestMessage{}
}

func (m DNSReadRequestMessage) Name() string {
	return "DNSReadRequestMessage"
}

func (m DNSReadRequestMessage) String() string {
	return fmt.Sprintf("DNSReadRequestMessage: Domain=%s", m.Domain)
}

func (m DNSReadRequestMessage) HTML() string {
	return fmt.Sprintf("<b>DNSReadRequestMessage</b>: Domain=%s", m.Domain)
}

// DNSReadReplyMessage represents a reply message for a DNS read request.
type DNSReadReplyMessage struct {
	Domain     string
	IPAddress  string
	TTL        time.Duration
	Owner      string
	Exists     bool
	Expiration time.Time
}

func (m DNSReadReplyMessage) NewEmpty() Message {
	return &DNSReadReplyMessage{}
}

func (m DNSReadReplyMessage) Name() string {
	return "DNSReadReplyMessage"
}

func (m DNSReadReplyMessage) String() string {
	return fmt.Sprintf("DNSReadReplyMessage: Domain=%s, IPAddress=%s, TTL=%s, Owner=%s, Expiration=%s", m.Domain, m.IPAddress, m.TTL, m.Owner, m.Expiration)
}

func (m DNSReadReplyMessage) HTML() string {
	return fmt.Sprintf("<b>DNSReadReplyMessage</b>: Domain=%s, IPAddress=%s, TTL=%s, Owner=%s, Expiration=%s", m.Domain, m.IPAddress, m.TTL, m.Owner, m.Expiration)
}

// Transaction types
type TransactionType string

const (
	NameNew         TransactionType = "name_new"
	NameFirstUpdate TransactionType = "name_firstupdate"
	NameUpdate      TransactionType = "name_update"
)

// Transaction input for UTXO
type TransactionInput struct {
	TransactionID string
	Index         int
}

type TransactionOutput struct {
	// For NameNew: this will store the hashed domain (not the plaintext)
	// For NameFirstUpdate/NameUpdate: this will store the actual domain name, now revealed
	DomainName string
	IP         string
	Owner      string
	Expiration time.Time
}

// For NameFirstUpdate, we need to reveal the salt and plaintext domain
type Transaction struct {
	ID           string
	Type         TransactionType
	Input        TransactionInput // For NameNew, this may be empty or a dummy input
	Output       TransactionOutput
	PlainDomain  string // Only used in NameFirstUpdate to reveal domain
	Salt         string // Only used in NameFirstUpdate to verify the hash
	HashedDomain string // For NameNew: store the hashed domain here instead of plaintext in Output.DomainName
}

// UTXO represents an unspent transaction output.
type UTXO struct {
	TransactionID string
	Index         int
	DomainName    string // This could be hashed domain for NameNew and actual domain for NameFirstUpdate/Update
	IP            string
	Owner         string
	Expiration    time.Time
}

// Block represents a block in the blockchain.
type Block struct {
	PrevBlockHash []byte
	Bits          int
	Nonce         int
	Transactions  []*Transaction
}

// TransactionMessage is a message containing a Transaction.
// Extends the types.Message interface (for handlers to be able to handle this message).
// It will be broadcast or unicast to other peers for them to add to their mempool.
type TransactionMessage struct {
	Tx Transaction
}

// NewEmpty implements types.Message.
func (TransactionMessage) NewEmpty() Message {
	return &TransactionMessage{}
}

// Name implements types.Message.
func (TransactionMessage) Name() string {
	return "transaction"
}

// String implements types.Message.
func (tm TransactionMessage) String() string {
	return fmt.Sprintf("TransactionMessage: ID=%s, Type=%s", tm.Tx.ID, tm.Tx.Type)
}

// HTML implements types.Message.
func (tm TransactionMessage) HTML() string {
	return fmt.Sprintf("<b>TransactionMessage</b>: ID=%s, Type=%s", tm.Tx.ID, tm.Tx.Type)
}
