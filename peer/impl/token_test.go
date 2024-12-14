package impl

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_Token_create(t *testing.T) {
	// Create two tokens with different values
	testToken1 := NewToken("TestToken1", "TT1", 0)
	testToken2 := NewToken("TestToken2", "TT2", 456)

	require.Equal(t, testToken1.Name, "TestToken1")
	require.Equal(t, testToken1.Shorthand, "TT1")
	require.Equal(t, testToken1.TotalSupply, uint64(0))

	require.Equal(t, testToken2.Name, "TestToken2")
	require.Equal(t, testToken2.Shorthand, "TT2")
	require.Equal(t, testToken2.TotalSupply, uint64(456))
}

func Test_Token_AddAddress(t *testing.T) {
	tt := NewToken("TestToken", "TT", 0)

	// Check that Balances is empty first
	require.Empty(t, tt.Balances.ToMap(), "Balances should be empty after creating a new token")

	// Add a new address with no initial balance
	tt.AddAddress("addr1234")
	require.Len(t, tt.Balances.ToMap(), 1) // Balances should have only 1 element in map
	balance, exists := tt.Balances.Get("addr1234")
	require.True(t, exists) // value exists
	require.Equal(t, balance, uint64(0))

	// Add another address with an arbitrary initial balance
	tt.AddAddress("addr5678", 2345)
	require.Len(t, tt.Balances.ToMap(), 2)
	balance, exists = tt.Balances.Get("addr5678")
	require.True(t, exists) // value exists
	require.Equal(t, balance, uint64(2345))

	// Adding already existing address should return an error
	err := tt.AddAddress("addr5678")
	require.Error(t, err, "Adding already existing address should return an error, but did not.")
	require.Len(t, tt.Balances.ToMap(), 2)
}

func Test_Token_Mint(t *testing.T) {
	tt := NewToken("TestToken", "TT", 0)

	// Mint to new address should add said address to the mapping
	tt.Mint("newAddress", 2456)
	balance, exists := tt.Balances.Get("newAddress")
	require.True(t, exists)
	require.Equal(t, balance, uint64(2456))
	require.Equal(t, tt.TotalSupply, uint64(2456))

	// Mint to existing address should add balance and supply
	tt.Mint("newAddress", 1000)
	balance, _ = tt.Balances.Get("newAddress")
	require.Equal(t, balance, uint64(3456))
	require.Equal(t, tt.TotalSupply, uint64(3456))
}

func Test_Token_GetBalance(t *testing.T) {
	tt := NewToken("TestToken", "TT", 0)
	tt.Mint("newAddress", 2456)

	// If adress does not exist, returns 0
	balance := tt.GetBalance("notAnAddress")
	require.Equal(t, balance, uint64(0))

	// Return expected balance
	balance = tt.GetBalance("newAddress")
	require.Equal(t, balance, uint64(2456))
}

func Test_Token_Burn(t *testing.T) {
	tt := NewToken("TestToken", "TT", 0)
	tt.Mint("newAddress", 1000)

	// Burn should return an error if address does not exist
	err := tt.Burn("notAnAddress", 50)
	require.Error(t, err, "Burn should return an error if address does not exist, but did not.")

	// Burn should return an error if we burn more than available balance
	err = tt.Burn("newAddress", 1001)

	// After burning, we should get expected balance and total supply
	err = tt.Burn("newAddress", 200)
	require.NoError(t, err, "Burn should not return an error, but it did.")
	balance := tt.GetBalance("newAddress")
	require.Equal(t, balance, uint64(800))
	require.Equal(t, tt.TotalSupply, uint64(800))
}

func Test_Token_Transfer(t *testing.T) {
	tt := NewToken("TestToken", "TT", 0)
	tt.Mint("addr1", 1000)
	tt.Mint("addr2", 500)

	// if "from" address does not exist, we should have an error
	err := tt.Transfer("notAnAddress", "addr1", 200)
	require.Error(t, err, "Transfer should return an error if `from` address does not exist, but did not.")

	// If "to" address does not exist, it should be added
	err = tt.Transfer("addr1", "addr3", 300)
	require.NoError(t, err, "Transfer to unexisting address should not return an error, but it did.")
	balance1 := tt.GetBalance("addr1")
	balance3 := tt.GetBalance("addr3")
	require.Equal(t, balance1, uint64(700))
	require.Equal(t, balance3, uint64(300))

	// If balance is too low for the transfer value, we should get an error
	err = tt.Transfer("notAnAddress", "addr3", 301)
	require.Error(t, err, "Transfer should return an error if insufficient balance, but did not.")

	// We get expected balance after transfer between two existing addresses
	err = tt.Transfer("addr2", "addr3", 50)
	require.NoError(t, err, "Transfer between existing addresses should not return an error, but it did.")

	balance1 = tt.GetBalance("addr1")
	balance2 := tt.GetBalance("addr2")
	balance3 = tt.GetBalance("addr3")
	require.Equal(t, balance1, uint64(700))
	require.Equal(t, balance2, uint64(450))
	require.Equal(t, balance3, uint64(350))

	// Total supply should match
	require.Equal(t, tt.TotalSupply, uint64(1500))
}
