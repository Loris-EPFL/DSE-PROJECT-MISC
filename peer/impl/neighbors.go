package impl

//getter of the neighbors
//iterates thru the routing table of n and returns everyone but the node itself
func (n *node) getNeighbors() []string {


    log := n.getLogger()

    size := n.directPeers.Size()

    neighbors := make([]string, 0, size)
    
    copyMap := n.directPeers.ToMap()

    for neighbor := range copyMap{
        neighbors = append(neighbors, neighbor)
    }

    if len(neighbors) == 0 {
        log.Warn().
            Msg("No neighbors available to broadcast")
            return nil
    }
    return neighbors
}



//function that returns all the neighbours of a node but one, which is the source of the rumor
func(n *node) getOtherNeighbors(exclude string)[]string {

    cp := n.directPeers.ToMap()
    length := n.directPeers.Size()

    //had access to n.directPeers.m directly, twice
	neighbors := make([]string, 0, length)
	for neighbor := range cp {
		if neighbor != exclude && neighbor != n.conf.Socket.GetAddress() {
			neighbors = append(neighbors, neighbor)
		}
	}
	return neighbors
   }