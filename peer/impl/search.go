package impl

import (
	"regexp"
	"strings"
	"time"

	"github.com/rs/xid"
	"go.dedis.ch/cs438/peer"
	"go.dedis.ch/cs438/transport"
	"go.dedis.ch/cs438/types"
)

// SearchAll returns all the names that exist matching the given regex. It
// merges results from the local storage and from the search request reply
// sent to a random neighbor using the provided budget. It makes the peer
// update its catalog and name storage according to the SearchReplyMessages
// received. Returns an empty result if nothing found. An error is returned
// in case of an exceptional event.
//
func (n *node) SearchAll(reg regexp.Regexp, budget uint, timeout time.Duration) (names []string, err error) {

	log := n.getLogger()

	//1. Get local filenames
	localFilenames := n.getLocalFilenamesMatching(&reg)
	log.Info().Msgf("Local filenames: %v", localFilenames)

	//2. Set up channels and requestID
	neighbors := n.getNeighbors()
	numNeighbors := len(neighbors)

	if numNeighbors == 0 {
		log.Info().Msg("no neighbors to search from")
		return localFilenames, nil
	}

	//prepare for search by allocating budget, creating a requestId and creating a channel for each neighbor the request
	//message is gonna be sent
	budgetAllocation := n.divideBudget(uint64(budget), uint64(numNeighbors))
	requestID := xid.New().String()
	replyChan := make(chan *types.SearchReplyMessage, numNeighbors)

	n.searchReplyChan.Add(requestID, replyChan)
	defer n.searchReplyChan.Remove(requestID)

	for i, neighbor := range neighbors {
		neighborBudget := budgetAllocation[i]
		if neighborBudget > 0 {
			n.sendSearchRequest(requestID, neighborBudget, neighbor, reg)
		}
	}

	collectedFilenames := make(map[string]struct{})

	for _, name := range localFilenames {
		collectedFilenames[name] = struct{}{}
	}

	for {
		select {
		case reply := <-replyChan:
			log.Info().Msgf("Received search reply: %v", reply)
			for _, fileInfo := range reply.Responses {
				collectedFilenames[fileInfo.Name] = struct{}{}
			}
			log.Info().Msgf("Collected filenames: %v", collectedFilenames)

		case <-time.After(timeout):

			result := make([]string, 0, len(collectedFilenames))
			for name := range collectedFilenames {
				result = append(result, name)
			}

			return result, nil
		}
	}

}

// Naming storage

// Name1 -> metaHash1
// Name2 -> metaHash2
// ...

// Blob
// metahash -> metaFileContent : (chunkHash1, chunkHash2, ...)
// ...
// chunkHash -> chunk

// SearchLocalFiles searches for local files matching the given regular expression.
// It retrieves file information for each matching file, including name, metahash, and chunks.
// Returns a slice of FileInfo for each matching file.
func (n *node) SearchLocalFiles(reg *regexp.Regexp) []types.FileInfo {

	log := n.getLogger()
	var files []types.FileInfo
	storage := n.conf.Storage.GetNamingStore()
	blob := n.conf.Storage.GetDataBlobStore()

	matchingNames := n.getLocalFilenamesMatching(reg)

	for _, name := range matchingNames {
		metaHash := storage.Get(name)
		metaFile := blob.Get(string(metaHash))

		if metaFile == nil || metaHash == nil {
			continue
		}

		var chunks [][]byte
		chunkHashes := strings.Split(string(metaFile), MetafileSep)
		chunks = make([][]byte, len(chunkHashes))

		for i, chunkHash := range chunkHashes {
			chunkData := []byte(chunkHash)

			if chunkData == nil || blob.Get(string(chunkData)) == nil {
				chunks[i] = nil
			} else {
				chunks[i] = chunkData
			}

		}
		
		newFileInfo := types.FileInfo{
			Name:     name,
			Metahash: string(metaHash),
			Chunks:   chunks}

		files = append(files, newFileInfo)
	}

	log.Info().Msgf("Local files: %v", files)
	return files
}

// getLocalFilenamesMatching returns a slice of local file names that match the given regular expression.
// Only includes files whose metahash exists in the blob store.
func (n *node) getLocalFilenamesMatching(reg *regexp.Regexp) []string {
	var matchingNames []string

	storage := n.conf.Storage.GetNamingStore()
	blob := n.conf.Storage.GetDataBlobStore()

	storage.ForEach(func(name string, metaHash []byte) bool {
		if reg.MatchString(name) {

			// Check if the metaHash exists in the blob store
			if blob.Get(string(metaHash)) != nil {
				matchingNames = append(matchingNames, name)
			}
		}
		return true
	})

	return matchingNames
}

// SearchFirst searches for the first fully known file matching the given regular expression.
// It first searches local files, and if not found, initiates an expanding ring search
// using the provided expanding ring configuration. Returns the name of the first fully known
// file found, or an empty string if none found.
func (n *node) SearchFirst(pattern regexp.Regexp, conf peer.ExpandingRing) (name string, err error) {
	log := n.getLogger()

	// Get local filenames
	if name, found := n.searchLocalFullyKnownFile(pattern); found {
		return name, nil
	}

	log.Info().Msg("Initiating expanding ring search")
	return n.ExpandingRingSearch(pattern, conf)
	//set up expanding ring search

}

// ExpandingRingSearch performs an expanding ring search for a file matching the given pattern.
// It sends search requests with increasing budgets until a matching file is found or retries are exhausted.
// Returns the name of the found file, or an empty string if none found.
func (n *node) ExpandingRingSearch(pattern regexp.Regexp, conf peer.ExpandingRing) (name string, err error) {
	log := n.getLogger()
	budget := conf.Initial

	for attempt := uint(0); attempt < conf.Retry; attempt++ {
		log.Info().Msgf("Attempt %d with budget %d", attempt+1, budget)

		requestID, replyChan := n.SendSearchRequests(budget, &pattern)
		if replyChan == nil {
			log.Warn().Msg("No neighbors to search from")
			break
		}
		defer n.searchReplyChan.Remove(requestID)

	Loop:
		for {
			select {
			case reply := <-replyChan:

				for _, fileInfo := range reply.Responses {
					if n.hasAllChunks(fileInfo) {
						return fileInfo.Name, nil
					}
				}
			case <-time.After(conf.Timeout):
				log.Info().Msgf("Timeout on attemopt %d", attempt+1)
				break Loop
			}

		}
		budget *= conf.Factor
	}

	return "", nil
}

// SendSearchRequests sends search requests to neighbors with allocated budgets.
// Returns the requestID and a channel to receive search replies.
func (n *node) SendSearchRequests(budget uint, pattern *regexp.Regexp) (string, chan *types.SearchReplyMessage) {

	requestID := xid.New().String()
	replyChan := make(chan *types.SearchReplyMessage, 50)
	n.searchReplyChan.Add(requestID, replyChan)

	neighbors := n.getNeighbors()
	numNeighbors := len(neighbors)
	if numNeighbors == 0 {
		return "", nil
	}

	budgetAllocation := n.divideBudget(uint64(budget), uint64(numNeighbors))
	for i, neighbor := range neighbors {
		neighborBudget := budgetAllocation[i]
		if neighborBudget > 0 {
			n.sendSearchRequest(requestID, neighborBudget, neighbor, *pattern)
		}
	}

	return requestID, replyChan
}

// searchLocalFullyKnownFile searches for a fully known file matching the given pattern in local storage.
// Returns the name of the first fully known file found and true, or empty string and false if not found.
func (n *node) searchLocalFullyKnownFile(pattern regexp.Regexp) (string, bool) {
	log := n.getLogger()
	localFiles := n.SearchLocalFiles(&pattern)

	for _, fileInfo := range localFiles {
		if n.hasAllChunks(fileInfo) {
			log.Info().Msgf("Found full file %s locally", fileInfo.Name)
			return fileInfo.Name, true
		}
	}
	return "", false
}

// hasAllChunks checks if all chunks are present for a file.
// Returns true if all chunks are present, false otherwise.
func (n *node) hasAllChunks(fileInfo types.FileInfo) bool {
	for _, chunk := range fileInfo.Chunks {
		if chunk == nil {
			return false
		}
	}
	return true
}

// sendForwardedSearchRequest forwards a search request to a destination with the given budget.
func (n *node) sendForwardedSearchRequest(searchReq *types.SearchRequestMessage, dest string, budget uint) {
	log := n.getLogger()

	fwSearchReq := &types.SearchRequestMessage{
		RequestID: searchReq.RequestID,
		Budget:    budget,
		Pattern:   searchReq.Pattern,
		Origin:    searchReq.Origin,
	}

	marsh, err := n.conf.MessageRegistry.MarshalMessage(fwSearchReq)
	if err != nil {
		return
	}
	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		dest,
	)

	pkt := createTransportPacket(&header, &marsh)

	err = n.conf.Socket.Send(dest, pkt, time.Second*1)
	if err != nil {
		log.Info().Msgf("Failed to forward search request to %s: %v", dest, err)
	}

}

// sendReplyMessage sends a search reply message with matching files back to the origin.
func (n *node) sendReplyMessage(matchingFiles []types.FileInfo,
	requestID string, origin string, source string) error {
	log := n.getLogger()

	searchReply := &types.SearchReplyMessage{
		RequestID: requestID,
		Responses: matchingFiles,
	}

	header := transport.NewHeader(
		n.conf.Socket.GetAddress(),
		n.conf.Socket.GetAddress(),
		origin,
	)

	replyPkt, err := n.conf.MessageRegistry.MarshalMessage(searchReply)

	if err != nil {
		log.Error().Msg("Failed to marshal SReplyMsg")
		return err
	}

	transpPkt := createTransportPacket(&header, &replyPkt)

	log.Debug().Msgf("Sending SearchReplyMessage with RequestID %s to %s", searchReply.RequestID, origin)

	err = n.conf.Socket.Send(source, transpPkt, time.Second*1)

	if err != nil {
		log.Error().Msgf("Failed to send SReplyMessage to %s", source)
		return err
	}

	return nil
}
