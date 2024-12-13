package impl

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

// Global logger function
func init() {
	// defaultLevel can be changed to set the desired level of the logger
	defaultLevel = zerolog.InfoLevel

	if os.Getenv("GLOG") == "no" {
		defaultLevel = zerolog.Disabled
	}

	logger = zerolog.New(logout).
		Level(defaultLevel).
		With().Timestamp().Logger().
		With().Caller().Logger().
		With().Str("role", "pow.go").Logger()
}

const MaxTxPerBlock = 3

func (n *node) startMIner() {
	n.wg.Add(1)
	go n.minerRoutine()
}

func (n *node) minerRoutine() {
	defer n.wg.Done()

	logger.Info().Msg("Starting miner routine")

	for {
		select {
		case <-n.stopCh:
			logger.Info().Msg("Stopping miner routine")
			return
		case <-n.newTxCh:
			n.attemptToMine()
		default:
			time.Sleep(time.Second)
		}
	}
}

func (n *node) attemptToMine() {

	logger.Info().Msg("Attempting to mine a new block")

	txs := n.getTxsFromMempool(MaxTxPerBlock)

	if len(txs) == 0 {
		logger.Info().Msg("No transactions to mine")
		return
	}

	candidateBlock := n.createCandidateBlock(txs)

	stopMiningCh := make(chan struct{})

	n.wg.Add(1)
	logger.Debug().Msgf("Entering Attempting to mine routine")
	go func() {
		defer n.wg.Done()
		for {

			select {
			case <-n.stopCh:
				close(stopMiningCh)
				return
			case <-n.newTxCh:
				//if a transaction arrives while mining
				if len(txs) < MaxTxPerBlock {
					close(stopMiningCh)
					return
				}
			default:
				time.Sleep(time.Second)
			}
		}
	}()

	minedBlock, err := n.mineBlock(candidateBlock, stopMiningCh)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to mine block")
		return
	}

	if minedBlock == nil {
		//mining stop because new txs arrived
		return
	}

	logger.Info().Msg("Block mined successfully")

	//Create block message
	blockMsg := types.BlockMessage{
		Block: *minedBlock,
	}

	transpBlockMsg, err := n.conf.MessageRegistry.MarshalMessage(&blockMsg)

	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal block message")
		return
	}

	//Broadcast the block
	err = n.Broadcast(transpBlockMsg)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to broadcast block")
		return
	}

	//Store the block in local blockchain storage
	err = n.addBlockToBlockchain(minedBlock)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to store block in blockchain")
		return
	}

	n.removeTxsFromMempool(minedBlock.Transactions)
}

func (n *node) createCandidateBlock(txs []types.Transaction) *types.Block {

	logger.Info().Msg("Creating candidate block")
	logger.Info().Msgf("Number of transactions: %d", len(txs))
	logger.Info().Msgf("Current nonce: %d", n.getCurrentNonce())
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

	challenge := 0
	target := n.computeTarget(0x1f111111)

	for {
		select {
		case <-stopMiningCh:
			logger.Info().Msg("Mining stopped due to new transactions")
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

func (n *node) computeTarget(bits int) *big.Int {

	logger.Debug().Msgf("Computing target with bits %d", bits)

	// Extract exponent (high byte) and coefficient (low 3 bytes)
	exponent := uint8(bits >> 24)                     // High byte of `bits`
	coefficient := big.NewInt(int64(bits & 0xFFFFFF)) // Low 24 bits

	target := new(big.Int)
	if exponent <= 3 {
		// Shift coefficient to the right for small exponents
		shift := (3 - exponent) * 8
		target.Rsh(coefficient, uint(shift))
	} else {
		// Shift coefficient to the left for large exponents
		shift := (exponent - 3) * 8
		target.Lsh(coefficient, uint(shift))
	}

	// Ensure the target is not zero
	if target.Sign() == 0 {
		logger.Error().Msg("Computed target is zero. Check the bits input.")
	}

	logger.Debug().Msgf("Computed target: %x", target)
	return target
}

func (n *node) getCurrentNonce() uint64 {
	// dummy implementation
	return uint64(n.currentHeight)
}

// PrintAllNonces prints the nonce of each block in the blockchain
func (n *node) PrintAllNonces() {

	bcStore := n.conf.Storage.GetBlockchainStore()

	// Get the last block's hash
	lastHash := bcStore.Get(storage.LastBlockKey)
	if len(lastHash) == 0 {
		logger.Println("No blocks found in the blockchain store.")
		return
	}

	currentHash := lastHash
	blockCount := 0

	for {
		// Convert the hash to a hexadecimal string to use as the key
		key := hex.EncodeToString(currentHash)

		// Retrieve the block data
		blockData := bcStore.Get(key)
		if len(blockData) == 0 {
			logger.Printf("Block data not found for hash: %s\n", key)
			break
		}

		// Deserialize the block (assuming JSON serialization)
		var block types.Block
		err := json.Unmarshal(blockData, &block)
		if err != nil {
			logger.Printf("Failed to deserialize block %s: %v\n", key, err)
			break
		}

		// Print the nonce
		fmt.Printf("Block %d - Hash: %s\n", block.Nonce, key)
		blockCount++

		// Check if we've reached the genesis block (assuming PrevBlockHash is all zeros or empty)
		if len(block.PrevBlockHash) == 0 || isAllZeros(block.PrevBlockHash) {
			break
		}

		// Move to the previous block
		currentHash = block.PrevBlockHash
	}

	if blockCount == 0 {
		fmt.Println("No blocks to display.")
	}
}

// isAllZeros checks if the given byte slice consists entirely of zero bytes
func isAllZeros(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}
