package impl

import (
	"encoding/hex"

	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
)


func (n * node) buildBlock(value string) (*types.BlockchainBlock, error) {



	// index := n.paxosState.currentStep

	// bcStore := n.conf.Storage.GetBlockchainStore()

	// prevHash := bcStore.Get(storage.LastBlockKey)

	// if prevHash == nil {
	// 	prevHash = make([]byte, 32)
	// }


	// block := &types.BlockchainBlock{
	// 	Index:    index,
	// 	Value:  *value,
	// 	PrevHash: prevHash,
	// }

	// hash:= n.computeBlockHash(block)

	// block.Hash = hash

	// return block, nil
	return nil, nil
}

func (n * node) computeBlockHash(block *types.BlockchainBlock) []byte{

	// indexStr := strconv.Itoa(int(block.Index))
	// data := append([]byte(indexStr), block.Value.Filename...)
	// data = append(data, block.Value.Metahash...)
	// data = append(data, block.PrevHash...)

	// hash := sha256.Sum256(data)
	// return hash[:]
	return nil
}



func (n * node) addBlockToBlockchain(block *types.BlockchainBlock) error {


	log:= n.getLogger()

	log.Debug().Msgf("Adding block %d to blockchain", block.Index)
	bcStore := n.conf.Storage.GetBlockchainStore()

	buf, err := block.Marshal()

	if err != nil {
		return err
	}

	hashHex := hex.EncodeToString(block.Hash)
	log.Debug().Msgf("Storing block %d with hash %s", block.Index, hashHex)
	bcStore.Set(hashHex, buf)
	log.Debug().Msgf("Updating last block hash to %s", storage.LastBlockKey)
	bcStore.Set(storage.LastBlockKey, block.Hash)
	return nil
}


