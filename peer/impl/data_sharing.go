package impl

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math/rand"
	"regexp"

	"strings"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
	"golang.org/x/xerrors"
)

 const MetafileSep = "\n";

// Upload stores a new data blob on the peer and will make it available to
	// other peers. The blob will be split into chunks.
	//
func (n * node) Upload(data io.Reader) (metahash string, err error) {

	log := n.getLogger()

	//first get the chunk size 
	chunkSize := n.conf.ChunkSize;


	if chunkSize <= 0 {
		log.Error().Msg("Chunk size <= than 0");
		return "", xerrors.Errorf("invalid chunk size: %d", chunkSize)
	}

	//read the file from the io.reader

	dataBytes, err := io.ReadAll(data)
	log.Debug().Msgf("Read %s", dataBytes);
	if err != nil {
		log.Error().Msg("Error reading data")
		return "", xerrors.Errorf("got %T", err);
	}

	//chunk splittage
	chunks := splitIntoChunks(dataBytes, chunkSize);

	chunksHashes := make([]string, len(chunks))
	blobStore := n.conf.Storage.GetDataBlobStore()

	//iterate over chunks, hash them, encode its hash and put it into the hasshes array, then set it in the blobstore
	//key - value here is chunckHash -> chunk
	for i, chunk := range chunks {
		hash := sha256.Sum256(chunk)

		chunkHash := hex.EncodeToString(hash[:])

		chunksHashes[i] = chunkHash

		blobStore.Set(chunkHash, chunk)
	}

	//Metafile generation and storage
	//key - value here is metaHash -> metaFileContent
	metafileContent := strings.Join(chunksHashes, MetafileSep)
	metaHashBytes := sha256.Sum256([]byte(metafileContent))
	metaHash := hex.EncodeToString(metaHashBytes[:])
	blobStore.Set(metaHash, []byte(metafileContent))

	//return the metaHash
	return metaHash, nil


	

//metahash -> metaFileContent : (chunkHash1, chunkHash2, ...)
//..
//chunkHash -> chunk

}

//Updates the catalog
func (n *node) UpdateCatalog(key string, peerAddr string){
	n.catalog.Update(key, peerAddr, n.conf.Socket.GetAddress())
}

//Gets the catalog
func (n *node) GetCatalog() peer.Catalog {
	return n.catalog.Get()
}


// Download will get all the necessary chunks corresponding to the given
	// metahash that references a blob, and return a reconstructed blob. The
	// peer will save locally the chunks that it doesn't have for further
	// sharing. Returns an error if it can't get the necessary chunks.
	//
func (n *node) Download(metahash string) ([]byte, error) {

	log := n.getLogger()

	//first we check for the metahash in our blobstore

	blobStore := n.conf.Storage.GetDataBlobStore()

	metafileContent := blobStore.Get(metahash)

	if metafileContent == nil {
		metafile, err := n.fetchWithBackOff(metahash)
		log.Warn().Msgf("Metafile content after fetching it: %s", metafileContent)
		log.Warn().Msgf("Metahash: %s", metahash)

		if err != nil {
            log.Error().Msgf("Failed to fetch metafile %s: %v", metahash, err)
			return nil, xerrors.Errorf("Failed to fetch metafile %s: %v", metahash, err)
		}
		metafileContent = metafile
	}

		blobStore.Set(metahash, metafileContent)

		chunkHashes := strings.Split(string(metafileContent), MetafileSep)
		chunks := make([][]byte, len(chunkHashes))

		for i, chunkHash := range chunkHashes {

			chunkData := blobStore.Get(chunkHash)

			if chunkData == nil {
				chunky, err := n.fetchWithBackOff(chunkHash)
				if err != nil {
					log.Error().Msgf("Failed to fetch chunk %s: %v", chunkHash, err)
					return nil, xerrors.Errorf("failed to fetch chunk %s: %v", chunkHash, err)
				}
				chunkData = chunky
			}
			blobStore.Set(chunkHash, chunkData)
			chunks[i] = chunkData
		}

	fileData := bytes.Join(chunks, []byte{})
	

	return fileData , nil
}


// Tag creates a mapping between a (file)name and a metahash.
	func (n * node) Tag(name string, metahash string) error {

		log:= n.getLogger()
		if name == "" || metahash == "" {
			return xerrors.Errorf("Name or metahash invalid")
		}

		storage := n.conf.Storage.GetNamingStore()

		existingValue := storage.Get(name)

		if existingValue != nil {
			return xerrors.Errorf("Name %s already exists", name)
		}

	
			log.Debug().Msg("No other peers, setting the metahash directly")
			storage.Set(name, []byte(metahash))
		return nil
	}



	// Resolve returns the corresponding metahash of a given (file)name. Returns
	// an empty string if not found.
	//
	func (n *node) Resolve(name string) string {
		log := n.getLogger()

		if name == "" {
			log.Error().Msg("Invalid name to be searched in the naming storage")
			return ""
		}
		storage := n.conf.Storage.GetNamingStore()
		
		val := storage.Get(name)


		return string(val)
	}

	




//* ****************
//* Helper functions
//* ****************
func splitIntoChunks(dataBytes []byte, chunkSize uint) [][]byte {

	var chunks [][]byte
	size := len(dataBytes)

	for i := 0 ; i < size; i += int(chunkSize){

		end := i  + int(chunkSize)
		if end > size{
			end = size
		}
		chunk := dataBytes[i:end]
		chunks = append(chunks, chunk) 
	}
	return chunks;
}



func (n * node) sendDataRequest(key string, destPeer string) ([]byte, error) {

	log := n.getLogger()

	requestID := xid.New().String()

	replyChan := make(chan *types.DataReplyMessage, 1)
	n.dataReplyChan.Add(requestID, replyChan)
	defer n.dataReplyChan.Remove(requestID) 



	dataRequest := &types.DataRequestMessage{
		RequestID: requestID,
		Key: key,
	}



	msgBytes, err := n.conf.MessageRegistry.MarshalMessage(dataRequest)

	if err != nil {
		log.Error().Msg("Error marshaling the data request message")
		return nil, err
	}

	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		destPeer,
	)

	

	nextHop, err := n.getNextHop(destPeer)

	if err != nil {
		return nil, err
	}
	

	pkt := createTransportPacket(&header, &msgBytes)


	initialTimeout := n.conf.BackoffDataRequest.Initial
	factor := n.conf.BackoffDataRequest.Factor
	maxRetries := int(n.conf.BackoffDataRequest.Retry)
	timeout := initialTimeout

	for retries := 0; retries < maxRetries; retries++ {

		err = n.conf.Socket.Send(nextHop, pkt, timeout)

		if err != nil {
			log.Error().Msgf("Error sending request, got: %s", err)
			return nil, err
		}	
		select{
			case dataReply := <-replyChan:
				
				if dataReply.RequestID == requestID {
					if dataReply.Value != nil {
						log.Warn().Msgf("Closing channel with request ID: %s", requestID)
						return dataReply.Value, nil

					}
					return nil, xerrors.Errorf("Peer %s does not have data", destPeer)
				}

			case <- time.After(timeout):
				timeout = time.Duration(float64(timeout) * float64(factor))
				continue
		}
	}

	return nil, xerrors.Errorf("timeout waiting for DataReplyMessage for key %s", key)
}



func (n * node) fetchWithBackOff(key string) ([]byte, error) {
	log := n.getLogger()
	log.Info().Msg("I am here")



	peers := n.catalog.GetPeersForKey(key)
	log.Debug().Msgf("Peers: %s and peers length: %d", peers, len(peers))

	if len(peers) == 0{
		return nil, xerrors.Errorf("No peers have data for key %s", key)
	}

	peersAddrs := make([]string, 0, len(peers))

	for peersAddr :=range peers {
		peersAddrs = append(peersAddrs, peersAddr) 
	}


	selectedPeer := getRandom(peersAddrs)

	var fetchedData []byte


	log.Warn().Msgf("Sending data request from %s to %s", n.conf.Socket.GetAddress(), selectedPeer)
	data, err := n.sendDataRequest(key, selectedPeer)
		if err == nil {
			expectedHash := key
			hash := sha256.Sum256(data)
			computedHash := hex.EncodeToString(hash[:])
			if computedHash != expectedHash{
				//see what to do, have to change 
				//is this expected
				n.catalog.Remove(key, selectedPeer)
				log.Info().Msgf("Data received don't match expected data for key %s", key)
				return nil, xerrors.Errorf("Data doesno't match expected data")
			}
			fetchedData = data
		}
	
	return fetchedData, nil

}




func (n * node) divideBudget(totalBudget, numNeighbors uint64) []uint64 {
	log := n.getLogger()

	budgets := make([]uint64, numNeighbors)

	if totalBudget == 0 || numNeighbors == 0{
		log.Error().Msg("Budget or neighbors not sufficient")
		return budgets
	}

	baseBudget := totalBudget / numNeighbors
	extraBudget := totalBudget % numNeighbors

	for i := range budgets {
		budgets[i] = baseBudget
	}


	indices := rand.Perm(int(numNeighbors))

	for i := uint64(0) ; i < extraBudget ; i++{
		budgets[indices[i]]++
	}

	return budgets
}


func (n * node) sendSearchRequest(requestID string, budget uint64, destPeer string, reg regexp.Regexp) {


	log := n.getLogger()

	searchRequestMsg := &types.SearchRequestMessage{
	 RequestID: requestID,
	 Budget: uint(budget),
	 Origin: n.conf.Socket.GetAddress(),
	 Pattern: reg.String(),
	}

	msgBytes, err := n.conf.MessageRegistry.MarshalMessage(searchRequestMsg)

	if err != nil {
		log.Error().Msgf("Unable to marshal message, got %s", err)
		return
	}

	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		destPeer,
	)

	pkt := createTransportPacket(&header, &msgBytes)


	nextHop, exists := n.routingTable.Get(destPeer)

	if !exists {
		log.Error().Msgf("No way to send message to %s from %s", destPeer, n.conf.Socket.GetAddress())
		return 
	}

	err = n.conf.Socket.Send(nextHop, pkt, time.Second * 1)

	if err != nil {
		log.Error().Msgf("Error while trying to send search request to %s Got: %v", destPeer, err)
	}
}

	







