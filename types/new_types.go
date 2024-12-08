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

// DNSUpdateMessage represents a message to update a DNS entry.
type DNSUpdateMessage struct {
	Domain     string
	IPAddress  string
	Expiration time.Time
	Owner      string
	Fee        uint64
}

func (m DNSUpdateMessage) NewEmpty() Message {
	return &DNSUpdateMessage{}
}

func (m DNSUpdateMessage) Name() string {
	return "DNSUpdateMessage"
}

func (m DNSUpdateMessage) String() string {
	return fmt.Sprintf("DNSUpdateMessage: Domain=%s,IPAddress=%s Expiration=%s, Owner=%s", m.Domain, m.IPAddress, m.Expiration, m.Owner)
}

func (m DNSUpdateMessage) HTML() string {
	return fmt.Sprintf("<b>DNSUpdateMessage</b>: Domain=%s,,IPAddress=%s Expiration=%s, Owner=%s", m.Domain, m.IPAddress, m.Expiration, m.Owner)
}

// DNSRegisterMessageNew represents a message to register a new DNS entry (At the 1st tx its only a salted hash to prevent frontrunning).
type DNSRegisterMessageNew struct {
	SaltedHash string
	Expiration time.Time
	Owner      string
	Fee        uint64
}

func (m DNSRegisterMessageNew) NewEmpty() Message {
	return &DNSRegisterMessageNew{}
}

func (m DNSRegisterMessageNew) Name() string {
	return "DNSRegisterMessageNew"
}

func (m DNSRegisterMessageNew) String() string {
	return fmt.Sprintf("DNSRegisterMessageNew: DomainHash=%s, Fee=%d, Expiration=%s, Owner=%s", m.SaltedHash, m.Fee, m.Expiration, m.Owner)
}

func (m DNSRegisterMessageNew) HTML() string {
	return fmt.Sprintf("<b>DNSRegisterMessageNew</b>: DomainHash=%s, Fee=%d, Expiration=%s,  Owner=%s", m.SaltedHash, m.Fee, m.Expiration, m.Owner)
}

// DNSRegisterMessageNew represents a message to register a new DNS entry (At the 1st tx its only a salted hash to prevent frontrunning).
type DNSRegisterMessageFirstUpdate struct {
	Domain     string
	IPAddress  string
	Expiration time.Time
	Owner      string
	Salt       string
	Fee        uint64
}

func (m DNSRegisterMessageFirstUpdate) NewEmpty() Message {
	return &DNSRegisterMessageFirstUpdate{}
}

func (m DNSRegisterMessageFirstUpdate) Name() string {
	return "DNSRegisterMessageFirstUpdate"
}

func (m DNSRegisterMessageFirstUpdate) String() string {
	return fmt.Sprintf("DNSRegisterMessageFirstUpdate: Domain=%s, IPAddress=%s, Expiration=%s, Owner=%s, Salt=%s , Fee=%d", m.Domain, m.IPAddress, m.Expiration, m.Owner, m.Salt, m.Fee)
}

func (m DNSRegisterMessageFirstUpdate) HTML() string {
	return fmt.Sprintf("<b>DNSRegisterMessageFirstUpdate</b>: Domain=%s, IPAddress=%s, Expiration=%s, Owner=%s, Salt=%s, Fee=%d", m.Domain, m.IPAddress, m.Expiration, m.Owner, m.Salt, m.Fee)
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
