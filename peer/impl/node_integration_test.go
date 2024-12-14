package impl

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	z "go.dedis.ch/cs438/internal/testing"
	"testing"
)

// Token related
func Test_Token_init(t *testing.T) {
	// The nct singleton should be initialized
	require.Equal(t, nct.Name, "Namecoin Token")
	require.Equal(t, nct.Shorthand, "NTC")
	require.Equal(t, nct.TotalSupply, uint64(0))
	require.Len(t, nct.Balances.ToMap(), 0)
}

func Test_Node_has_wallet(t *testing.T) {
	transp := channelFac()
	node1 := z.NewTestNode(t, peerFac, transp, "127.0.0.1:1")

	// Address should exist in token balance mapping
	assert.Len(t, nct.Balances.ToMap(), 1)
	balance := nct.GetBalance(node1.GetAddr())
	assert.Equal(t, balance, uint64(0))

	// After changes we still get expected value
	nct.Mint(node1.GetAddr(), 200)
	balance = nct.GetBalance(node1.GetAddr())
	assert.Equal(t, balance, uint64(200))
	err := nct.Burn(node1.GetAddr(), 200)
	require.NoError(t, err)
	balance = nct.GetBalance(node1.GetAddr())
	assert.Equal(t, balance, uint64(0))
}
