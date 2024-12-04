package types

import (
	"fmt"
	"time"
)

// DNSReadMessage represents a message to read a DNS entry.
type DNSReadMessage struct {
	Domain string
}

func (m DNSReadMessage) NewEmpty() Message {
	return &DNSReadMessage{}
}

func (m DNSReadMessage) Name() string {
	return "DNSReadMessage"
}

func (m DNSReadMessage) String() string {
	return fmt.Sprintf("DNSReadMessage: Domain=%s", m.Domain)
}

func (m DNSReadMessage) HTML() string {
	return fmt.Sprintf("<b>DNSReadMessage</b>: Domain=%s", m.Domain)
}

// DNSRenewalMessage represents a message to renew a DNS entry.
type DNSRenewalMessage struct {
	Domain     string
	Expiration time.Time
}

func (m DNSRenewalMessage) NewEmpty() Message {
	return &DNSRenewalMessage{}
}

func (m DNSRenewalMessage) Name() string {
	return "DNSRenewalMessage"
}

func (m DNSRenewalMessage) String() string {
	return fmt.Sprintf("DNSRenewalMessage: Domain=%s, Expiration=%s", m.Domain, m.Expiration)
}

func (m DNSRenewalMessage) HTML() string {
	return fmt.Sprintf("<b>DNSRenewalMessage</b>: Domain=%s, Expiration=%s", m.Domain, m.Expiration)
}

// DNSRegisterMessage represents a message to register a new DNS entry.
type DNSRegisterMessage struct {
	Domain     string
	IPAddress  string
	Expiration time.Time
}

func (m DNSRegisterMessage) NewEmpty() Message {
	return &DNSRegisterMessage{}
}

func (m DNSRegisterMessage) Name() string {
	return "DNSRegisterMessage"
}

func (m DNSRegisterMessage) String() string {
	return fmt.Sprintf("DNSRegisterMessage: Domain=%s, IPAddress=%s, Expiration=%s", m.Domain, m.IPAddress, m.Expiration)
}

func (m DNSRegisterMessage) HTML() string {
	return fmt.Sprintf("<b>DNSRegisterMessage</b>: Domain=%s, IPAddress=%s, Expiration=%s", m.Domain, m.IPAddress, m.Expiration)
}
