package rpcclient

import (
	"time"
)

const defaultProbeInterval = time.Second * 3

type Network struct {
	NetworkID string
	Nodes     map[NodeID]*Node
	Genesis   string
}

type Node struct {
	ID          NodeID
	JsonInfo    NodeInfo
	RPCURLs     map[string]bool
	PrefRPCURL  string
	RPCPort     int
	Peers       map[NodeID]Peer
	IsReachable bool
	ticker      *time.Ticker

	BlockNumber int64
	LastSeen    time.Time
	Txpool      []TxpoolTransaction
	Syncing     bool
	probed      bool
	Client      string
}

func (nt *Network) IsTrailing(nd *Node) bool {
	for _, n := range nt.Nodes {
		if n.BlockNumber-nd.BlockNumber > 2 {
			return true
		}
	}
	return false
}

func (n *Node) FormattedLastSeen() string {
	if n.LastSeen.IsZero() {
		return "Never"
	}
	return n.LastSeen.Format(time.RFC822)
}

type Peer struct {
	ID    NodeID
	PInfo PeerInfo
}

type NodeID string

func (nid *NodeID) Short() string {
	s := string(*nid)
	if len(s) == 0 {
		return "unknown"
	}
	return s[:8] + "..." + s[len(s)-8:]
}

func FillNodeFromNodeInfo(n *Node, ni *NodeInfo) *Node {
	n.JsonInfo = *ni
	n.ID = NodeID(ni.ID)
	n.RPCURLs[ni.IP] = false
	return n

}
