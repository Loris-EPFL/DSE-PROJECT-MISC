package impl

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"go.dedis.ch/cs438/storage"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

func (n *node) computeBlockHash(block *types.Block) []byte {

	// Hash is SHA256(Challenge || []Txs || Prevhash)
	// use crypto/sha256
	// Concatenate the Challenge, previous block hash, and all transactions
	// into a single byte slice

	indexStr := strconv.Itoa(int(block.Challenge))
	var txData []byte
	for _, tx := range block.Transactions {
		txBytes, err := tx.Marshal()
		if err != nil {
			return nil
		}
		txData = append(txData, txBytes...)
	}
	data := append([]byte(indexStr), txData...)
	data = append(data, block.PrevBlockHash...)

	hash := sha256.Sum256(data)
	return hash[:]
}

func (n *node) addBlockToBlockchain(block *types.Block) error {
	log := n.getLogger()

	var genesis bool
	if block.Nonce == 0 {
		genesis = true
	}

	if !genesis {
		for _, tx := range block.Transactions {

			switch tx.Type {
			case types.NameNew:
				// NameNew: produce UTXO with hashed domain
				newUTXO := types.UTXO{
					DomainName: tx.Output.DomainName, // This should be the hashed domain
					IP:         tx.Output.IP,
					Owner:      tx.Output.Owner,
					Expiration: tx.Output.Expiration,
				}
				n.addUTXO(tx.Output.DomainName, newUTXO)

			case types.NameFirstUpdate:
				// NameFirstUpdate: consume the NameNew UTXO
				inputUTXO, found, err := n.findUTXO(tx.Input)
				if err != nil || !found {
					log.Error().Err(err).Any("inputUTXO", inputUTXO).Msg("Input UTXO not found for NameFirstUpdate")
					return xerrors.Errorf("Input UTXO not found for NameFirstUpdate")
				}

				log.Info().Msgf("Removing UTXO: %s", tx.Input.DomainName)

				n.removeUTXO(tx.Input.DomainName)
				// Produce a new UTXO with the revealed domain name
				newUTXO := types.UTXO{
					DomainName: tx.Output.DomainName, // Now the actual domain name
					IP:         tx.Output.IP,
					Owner:      tx.Output.Owner,
					Expiration: tx.Output.Expiration,
				}
				n.addUTXO(tx.Output.DomainName, newUTXO)

			case types.NameUpdate:
				// NameUpdate: consume existing domain UTXO
				inputUTXO, found, err := n.findUTXO(tx.Input)
				if err != nil || !found {
					log.Error().Err(err).Any("inputUTXO", inputUTXO).Msg("Input UTXO not found for NameUpdate")
					return xerrors.Errorf("Input UTXO not found for NameUpdate")
				}
				n.removeUTXO(tx.Input.DomainName)

				// Produce updated UTXO
				newUTXO := types.UTXO{
					DomainName: tx.Output.DomainName,
					IP:         tx.Output.IP,
					Owner:      tx.Output.Owner,
					Expiration: tx.Output.Expiration,
				}
				n.addUTXO(tx.Output.DomainName, newUTXO)

			default:
			}
		}
	}
	//Store the block in local blockchain storage (not implemented here)
	//e.g.:
	serializedBlock, _ := block.Marshal()
	bcStore := n.conf.Storage.GetBlockchainStore()
	hashHex := hex.EncodeToString(block.Hash)
	bcStore.Set(hashHex, serializedBlock)
	bcStore.Set(storage.LastBlockKey, block.Hash)

	// Update the current height
	n.currentHeight++

	log.Info().Msgf("Block added to blockchain: %s", hashHex)
	log.Info().Msg("Block added and UTXO set updated successfully")

	n.PrintAllNonces()
	return nil
}

func (n *node) validateBlock(block *types.Block) error {
	//The block must have at least one transaction
	if len(block.Transactions) == 0 {
		return xerrors.New("Block must have at least one transaction")
	}
	// Additional POW checks, Merkle root checks, etc. go here.

	for i, tx := range block.Transactions {
		// Check if the transaction is valid
		if err := n.validateTransaction(tx); err != nil {
			return xerrors.Errorf("Invalid transaction %d in block: %v", i, err)
		}
	}

	return nil
}

func (n *node) validateTransaction(tx *types.Transaction) error {
	// don't forget to add fees and tokens check
	// Common checks
	log := n.getLogger()
	if tx.ID == "" {
		return xerrors.New("Transaction ID is empty")
	}

	switch tx.Type {
	case types.NameNew:
		if len(tx.Output.DomainName) == 0 {
			return xerrors.New("Hashed domain is empty in NameNew")
		}

		log.Info().Msgf("Domain name: %s", tx.HashedDomain)
		// Check if domain already exists by scanning UTXO
		// This prevents duplicate domain claims
		utxoSet := n.UTXOSet.ToMap()
		for _, utxo := range utxoSet {
			if utxo.DomainName == tx.Output.DomainName {
				return xerrors.New("Domain (hashed) already exists")
			}
		}

		// Check expiration is in the future
		if tx.Output.Expiration.Before(time.Now()) {
			return xerrors.New("Expiration must be in the future for NameNew")
		}
	case types.NameFirstUpdate:
		// Validate NameFirstUpdate
		// Must have a valid input referencing a NameNew UTXO
		inputUTXO, found, err := n.findUTXO(tx.Input)
		if err != nil || !found {
			return xerrors.Errorf("Input UTXO not found for NameFirstUpdate")
		}

		// Verify that the domain revealed matches the hashed domain in NameNew
		hashed := hashDomain(tx.Salt, tx.PlainDomain)

		nameNewUtxo, found, err := n.findUTXO(types.UTXO{DomainName: hashed})
		log.Debug().Msgf("NameNew UTXO: %v", nameNewUtxo)
		log.Info().Any("namenew hashed domain", nameNewUtxo.DomainName).Msgf("Computed hashed domain: %s", hashed)

		if !found {
			return err
		}

		// Ensure output domain is the revealed plaintext domain
		if tx.Output.DomainName != tx.PlainDomain {
			return xerrors.New("Output domain name must match the revealed plaintext domain")
		}

		// Expiration must be greater than the old expiration
		if tx.Output.Expiration.Before(inputUTXO.Expiration) {
			return xerrors.New("NameFirstUpdate expiration must be greater than NameNew expiration")
		}

		// Check expiration is in the future
		if tx.Output.Expiration.Before(time.Now()) {
			return xerrors.New("NameFirstUpdate expiration must be in the future")
		}

	case types.NameUpdate:
		// Validate NameUpdate
		inputUTXO, found, err := n.findUTXO(tx.Input)
		if err != nil || !found {
			return xerrors.New("Input UTXO not found for NameUpdate")
		}

		// Must keep the same domain name but can change IP/Expiration
		if tx.Output.DomainName != inputUTXO.DomainName {
			return xerrors.New("NameUpdate cannot change the domain name")
		}

		// New expiration must be greater than old expiration
		if tx.Output.Expiration.Before(inputUTXO.Expiration) {
			return xerrors.New("NameUpdate expiration must be greater than previous expiration")
		}

		// Check expiration is in the future
		if tx.Output.Expiration.Before(time.Now()) {
			return xerrors.New("NameUpdate expiration must be in the future")
		}
	default:
		return xerrors.New("Invalid transaction type")

	}
	return nil
}

// * auxiliary functions
func (n *node) findUTXO(utxoToFind types.UTXO) (types.UTXO, bool, error) {
	utxoSet := n.UTXOSet.ToMap()

	for _, utxo := range utxoSet {
		if utxo.DomainName == utxoToFind.DomainName && utxo.Owner == utxoToFind.Owner {
			return utxo, true, nil
		}
	}
	return types.UTXO{}, false, xerrors.New("Input UTXO not found")
}

func (n *node) addUTXO(domain string, utxo types.UTXO) {
	// Just add it to the UTXO set
	n.UTXOSet.Add(domain, utxo)

}

func (n *node) removeUTXO(domainName string) {
	n.UTXOSet.Remove(domainName)
}

func hashDomain(salt, domain string) string {
	h := sha256.New()
	h.Write([]byte(salt + domain))
	return hex.EncodeToString(h.Sum(nil))
}

//Proof of work

// this function will create the first block and broadcast it to all the blocks.
func (n *node) createGenesisBlock() *types.Block {
	// Create a new block

	//Create hash for the block
	hash := sha256.Sum256([]byte(storage.LastBlockKey))

	block := &types.Block{

		PrevBlockHash: hash[:],
		Nonce:         1,
		Transactions:  []*types.Transaction{},
	}

	blockHash := n.computeBlockHash(block)
	block.Hash = blockHash
	n.addBlockToBlockchain(block)

	return block
}

//4891456abc287282f533534c60191e329cfe4fa5cd91c21ffc959a6d7c9c2edb
//4891456abc287282f533534c60191e329cfe4fa5cd91c21ffc959a6d7c9c2edb
