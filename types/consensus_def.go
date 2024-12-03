package types

import (
	"encoding/json"
	"fmt"
	"io"

)



// BlockchainBlock defines the content of a block in the blockchain.
type BlockchainBlock struct {
	// Index is the index of the block in the blockchain, starting at 0 for the
	// first block.
	Index uint

	// Hash is SHA256(Index || v.Filename || v.Metahash || Prevhash)
	// use crypto/sha256
	Hash []byte

	// Value is the data stored in the block
	Value map[string]string

	// PrevHash is the SHA256 hash of the previous block
	PrevHash []byte
}

// Marshal marshals the BlobkchainBlock into a byte representation. Must be used
// to store blocks in the blockchain store.
func (b *BlockchainBlock) Marshal() ([]byte, error) {
	return json.Marshal(b)
}

// Unmarshal unmarshals the data into the current instance. To unmarshal a
// block:
//
//	var block BlockchainBlock
//	err := block.Unmarshal(buf)
func (b *BlockchainBlock) Unmarshal(data []byte) error {
	return json.Unmarshal(data, b)
}

// String returns a string representation of a blokchain block.
func (b BlockchainBlock) String() string {
	return fmt.Sprintf("{block n°%d H(%x) - %v - %x", b.Index, b.Hash[:4],
		b.Value, b.PrevHash[:4])
}

// DisplayBlock writes a rich string representation of a block
func (b BlockchainBlock) DisplayBlock(out io.Writer) {
	// crop := func(s string) string {
	// 	if len(s) > 10 {
	// 		return s[:8] + "..."
	// 	}
	// 	return s
	// }

	// maximum := func(s ...string) int {
	// 	m := 0
	// 	for _, se := range s {
	// 		if len(se) > m {
	// 			m = len(se)
	// 		}
	// 	}
	// 	return m
	// }

	// pad := func(n int, s ...*string) {
	// 	for _, se := range s {
	// 		*se = fmt.Sprintf("%-*s", n, *se)
	// 	}
	// }


	// row1 := fmt.Sprintf("%d | %x", b.Index, b.Hash[:6])
	// row2 := fmt.Sprintf("F | %s", crop(b.Value.Filename))
	// row3 := fmt.Sprintf("M | %s", crop(b.Value.Metahash))
	// row4 := fmt.Sprintf("<- %x", b.PrevHash[:6])

	// m := maximum(row1, row2, row3, row4)
	// pad(m, &row1, &row2, &row3, &row4)

	// fmt.Fprintf(out, "\n┌%s┐\n", strings.Repeat("─", m+2))
	// fmt.Fprintf(out, "│ %s │\n", row1)
	// fmt.Fprintf(out, "│%s│\n", strings.Repeat("─", m+2))
	// fmt.Fprintf(out, "│ %s │\n", row2)
	// fmt.Fprintf(out, "│ %s │\n", row3)
	// fmt.Fprintf(out, "│ %s │\n", row4)
	// fmt.Fprintf(out, "│%s│\n", strings.Repeat("─", m+2))
	// fmt.Fprintf(out, "└%s┘\n", strings.Repeat("─", m+2))
}
