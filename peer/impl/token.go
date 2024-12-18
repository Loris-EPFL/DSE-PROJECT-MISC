package impl

import (
	"fmt"
	"sync"
)

// TODO Use node logger (zerolog) once implementation is more functional to ensure consistent logging practices.

// Token represents a simple token system.
type Token struct {
	Name        string                  // Name of the token, e.g., "NameCoin Token"
	Shorthand   string                  // Shorthand of the token, e.g., "NCT"
	TotalSupply uint64                  // Total number of tokens in existence (optional field?)
	Balances    SafeMap[string, uint64] // Mapping from addresses to their token balances
	mu          sync.Mutex
}

// Global Token instance
var (
	nct  *Token
	once sync.Once
)

// Initialize the Token in the init function
func init() {
	once.Do(func() {
		nct = NewToken("Namecoin Token", "NTC", 0)
	})
}

// NewToken initializes a new Token with the given name, symbol, and initial supply.
func NewToken(name, symbol string, initialSupply uint64) *Token {
	return &Token{
		Name:        name,
		Shorthand:   symbol,
		TotalSupply: initialSupply, // May not be needed if we only mint and burn anyway (set to 0).
		Balances:    NewSafeMap[string, uint64](),
	}
}

// AddAddress adds a new address with an optional initial balance.
// If no balance is provided, it defaults to 0.
func (t *Token) AddAddress(address string, initialBalance ...uint64) error {

	if len(initialBalance) > 1 {
		return fmt.Errorf("AddAddress: too many arguments for initialBalance")
	}

	var balance uint64 = 0
	if len(initialBalance) == 1 {
		balance = initialBalance[0]
	}

	if t.Balances.Exists(address) {
		return fmt.Errorf("AddAddress: address %s already exists", address)
	}

	t.Balances.Add(address, balance)

	t.mu.Lock()
	t.TotalSupply += balance
	t.mu.Unlock()
	fmt.Printf("Added address %s with balance %d\n", address, balance)
	return nil
}

// Mint adds tokens to a specified address and increases the total supply.
func (t *Token) Mint(address string, amount uint64) {
	currentBalance, exists := t.Balances.Get(address)
	if !exists {
		// If address does not exist, add it with the minted amount
		t.Balances.Add(address, amount)
	} else {
		// Update the balance
		t.Balances.Add(address, currentBalance+amount)
	}

	t.mu.Lock()
	t.TotalSupply += amount
	t.mu.Unlock()

	fmt.Printf("Minted %d tokens to %s. Total Supply: %d\n", amount, address, t.TotalSupply)
}

// Burn removes tokens from an address and decreases the total supply.
func (t *Token) Burn(address string, amount uint64) error {

	currentBalance, exists := t.Balances.Get(address)
	if !exists {
		return fmt.Errorf("Burn: address %s does not exist", address)
	}
	if currentBalance < amount {
		return fmt.Errorf("Burn: insufficient balance in address %s", address)
	}

	// Update the balance
	t.Balances.Add(address, currentBalance-amount)

	t.mu.Lock()
	t.TotalSupply -= amount
	t.mu.Unlock()

	fmt.Printf("Burned %d tokens from %s. Total Supply: %d\n", amount, address, t.TotalSupply)
	return nil
}

// Transfer moves tokens from one address to another.
func (t *Token) Transfer(from, to string, amount uint64) error {
	fromBalance, exists := t.Balances.Get(from)
	if !exists {
		return fmt.Errorf("Transfer: address %s does not exist", from)
	}
	if fromBalance < amount {
		return fmt.Errorf("Transfer: insufficient balance in address %s", from)
	}

	toBalance, exists := t.Balances.Get(to)
	if !exists {
		// If 'to' address does not exist, add it with the transferred amount
		t.Balances.Add(to, amount)
	} else {
		// Update the balance
		t.Balances.Add(to, toBalance+amount)
	}

	// Update 'from' balance
	t.Balances.Add(from, fromBalance-amount)

	fmt.Printf("Transferred %d tokens from %s to %s\n", amount, from, to)
	return nil
}

// GetBalance retrieves the token balance of a given address.
func (t *Token) GetBalance(address string) uint64 {
	balance, exists := t.Balances.Get(address)
	if !exists {
		return 0
	}
	return balance
}
