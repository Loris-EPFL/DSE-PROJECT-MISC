package impl

import (
	"math/big"
	"time"

	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

const MaxTxPerBlock = 3

func (n *node) startMIner() {
	n.wg.Add(1)
	go n.minerRoutine()
}

func (n *node) minerRoutine() {
	defer n.wg.Done()
	log := n.getLogger()
	log.Info().Msg("Starting miner routine")

	for {
		select {
		case <-n.stopCh:
			log.Info().Msg("Stopping miner routine")
			return
		case <-n.newTxCh:
			n.attemptToMine()
		default:
			time.Sleep(time.Second)
		}
	}
}

func (n *node) attemptToMine() {
	log := n.getLogger()
	log.Info().Msg("Attempting to mine a new block")

	txs := n.getTxsFromMempool(MaxTxPerBlock)

	if len(txs) == 0 {
		log.Info().Msg("No transactions to mine")
		return
	}

	candidateBlock := n.createCandidateBlock(txs)

	stopMiningCh := make(chan struct{})

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		for {

			select {
			case <-n.stopCh:
				close(stopMiningCh)
			case <-n.newTxCh:
				//if a transaction arrives while mining
				if len(txs) < MaxTxPerBlock {
					close(stopMiningCh)
				}
			}
		}
	}()

	minedBlock, err := n.mineBlock(candidateBlock, stopMiningCh)
	if err != nil {
		log.Error().Err(err).Msg("Failed to mine block")
		return
	}

	if minedBlock == nil {
		//mining stop because new txs arrived
		return
	}

	log.Info().Msg("Block mined successfully")

	//Create block message
	blockMsg := types.BlockMessage{
		Block: *minedBlock,
	}

	transpBlockMsg, err := n.conf.MessageRegistry.MarshalMessage(&blockMsg)

	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal block message")
		return
	}

	//Broadcast the block
	err = n.Broadcast(transpBlockMsg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to broadcast block")
		return
	}

	//Store the block in local blockchain storage
	err = n.addBlockToBlockchain(minedBlock)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store block in blockchain")
		return
	}

	n.removeTxsFromMempool(minedBlock.Transactions)

}

func (n *node) createCandidateBlock(txs []types.Transaction) *types.Block {

	prevHash := n.getLastBlockHash()
	nonce := n.getCurrentNonce()
	newNonce := nonce + 1
	block := &types.Block{
		PrevBlockHash: prevHash,
		Transactions:  txSliceToPtr(txs),
		Nonce:         int(newNonce),
	}
	return block
}

// txSliceToPtr converts a slice of Transaction to a slice of *Transaction
func txSliceToPtr(txs []types.Transaction) []*types.Transaction {
	res := make([]*types.Transaction, len(txs))
	for i := range txs {
		res[i] = &txs[i]
	}
	return res
}


func (n *node) mineBlock(block *types.Block, stopMiningCh chan struct{}) (*types.Block, error) {
	log := n.getLogger()

	challenge := 0
	target := n.computeTarget(int(n.conf.PowBits))

	for {
		select {
		case <-stopMiningCh:
			log.Info().Msg("Mining stopped due to new transactions")
			return nil, nil
		case <-n.stopCh:
			return nil, xerrors.New("Node shutting down")
		default:
			block.Challenge = challenge
			hash := n.computeBlockHash(block)
			block.Hash = hash
			if n.meetsTarget(hash, target) {
				return block, nil
			}
			challenge++
		}
	}
}

// getMempoolTransactions returns up to 'limit' transactions from mempool
func (n *node) getTxsFromMempool(limit int) []types.Transaction {
	m := n.mempool.ToMap()
	txs := make([]types.Transaction, 0, len(m))
	count := 0
	for _, tx := range m {
		txs = append(txs, tx)
		count++
		if count >= limit {
			break
		}
	}
	return txs
}

func (n *node) removeTxsFromMempool(txs []*types.Transaction) {
	toRemove := make(map[string]struct{})
	for _, tx := range txs {
		toRemove[tx.ID] = struct{}{}
	}

	all := n.mempool.ToMap()
	for id := range all {
		if _, found := toRemove[id]; found {
			n.mempool.Remove(id)
		}
	}
}

func (n *node) getLastBlockHash() []byte {
	bcStore := n.conf.Storage.GetBlockchainStore()
	return bcStore.Get(storage.LastBlockKey)
}

// meetsTarget checks if the hash is under the target
func (n *node) meetsTarget(hash []byte, target *big.Int) bool {

	hashValue := new(big.Int).SetBytes(hash)

	// hashValue must be <= target for a valid block.
	return hashValue.Cmp(target) <= 0
}



// computeTarget computes the target given bits
func (n *node) computeTarget(bits int) *big.Int {

	
	exponent := uint8(bits >> 24)
	coefficient := big.NewInt(int64(bits & 0xffffff))

	target := new(big.Int)
	// target = coefficient * 256^(exponent-3)
	exp := exponent - 3
	if exponent <= 3 {
		// If exponent <= 3, shift the coefficient right
		// Because if exponent < 3, we have a situation where
		// we just shift coefficient down
		shift := (3 - exponent) * 8
		target.Rsh(coefficient, uint(shift))
	} else {
		// If exponent > 3, shift left
		shift := (exp * 8)
		target.Lsh(coefficient, uint(shift))
	}

	return target
}

func (n *node) getCurrentNonce() uint64 {
	// dummy implementation
	return uint64(n.currentHeight)
}
