package types

import (
	"encoding/json"
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

type BlockMessage struct {
	Block Block
}

func (m BlockMessage) NewEmpty() Message {
	return &BlockMessage{}
}

func (m BlockMessage) Name() string {
	return "BlockMessage"
}

func (m BlockMessage) String() string {
	return fmt.Sprintf("BlockMessage: PrevBlockHash=%x, Nonce=%d, Transactions=%v", m.Block.PrevBlockHash, m.Block.Nonce, m.Block.Transactions)
}

func (m BlockMessage) HTML() string {
	return fmt.Sprintf("<b>BlockMessage</b>: PrevBlockHash=%x, Nonce=%d, Transactions=%v", m.Block.PrevBlockHash,
		m.Block.Nonce, m.Block.Transactions)
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

// For NameFirstUpdate, we need to reveal the salt and plaintext domain
type Transaction struct {
	ID           string
	Type         TransactionType
	Input        UTXO // For NameNew, this may be empty or a dummy input
	Output       UTXO
	PlainDomain  string // Only used in NameFirstUpdate to reveal domain
	Salt         string // Only used in NameFirstUpdate to verify the hash
	HashedDomain string // For NameNew: store the hashed domain here instead of plaintext in Output.DomainName
	Fees         uint
}

// UTXO represents an unspent transaction output.
type UTXO struct {
	DomainName    string // This could be hashed domain for NameNew and actual domain for NameFirstUpdate/Update
	IP            string
	Owner         string
	Expiration    time.Time
}

// Block represents a block in the blockchain.
type Block struct {
	// Hash is SHA256(Nonce || Prevhash || []Txs)
	// use crypto/sha256
	Hash          []byte
	PrevBlockHash []byte
	Nonce         int
	Challenge     int
	Transactions  []*Transaction
}

func (tx *Transaction) Marshal() ([]byte, error) {
	return json.Marshal(tx)
}

func (tx *Transaction) Unmarshal(data []byte) error {
	return json.Unmarshal(data, tx)
}

// Marshal marshals the BlobkchainBlock into a byte representation. Must be used
// to store blocks in the blockchain store.
func (b *Block) Marshal() ([]byte, error) {
	return json.Marshal(b)
}

// Unmarshal unmarshals the data into the current instance. To unmarshal a
// block:
//
//	var block BlockchainBlock
//	err := block.Unmarshal(buf)
func (b *Block) Unmarshal(data []byte) error {
	return json.Unmarshal(data, b)
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
