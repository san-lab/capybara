package rpcclient

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/san-lab/capybara/templates"
)

const keyword_action = "action"
const keyword_nodeid = "nodeid"
const keyword_blocknum = "blocknum"

func (rpcClient *Client) GetNetwork(rpcendpoint string) (*string, error) {
	data := rpcClient.NewCallData("net_version")
	data.Context.TargetRPCEndpoint = rpcendpoint

	net := new(string)
	err := rpcClient.actualRpcCall(data, net)
	if err != nil {
		return nil, err
	}
	return net, nil
}

func (rpcClient *Client) GetNodeInfo(rpcendpoint string) (*NodeInfo, error) {
	data := rpcClient.NewCallData("admin_nodeInfo")
	data.Context.TargetRPCEndpoint = rpcendpoint
	ni := new(NodeInfo)
	err := rpcClient.actualRpcCall(data, ni)
	return ni, err
}

func (rpcClient *Client) DirectMethod(w http.ResponseWriter, rq *http.Request) (err error) {
	fmt.Println("Direct method call")
	rq.ParseForm()
	meth := rq.Form.Get("method")
	if len(meth) > 0 {
		data := rpcClient.NewCallData(meth)
		data.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
		err := rpcClient.actualRpcCall(data, new(string))
		fmt.Fprintln(w, "Error: ", err)
		fmt.Fprintln(w, string(data.Response.Result))
	} else {
		fmt.Fprintln(w, "No method supplied")
	}
	return err
}

func (rpcClient *Client) Initialize(rpcendpoint string) error {
	fmt.Println("initializing")
	net, err := rpcClient.GetNetwork(rpcendpoint)
	if err != nil {
		fmt.Println(err)
		return err
	}
	rpcClient.Model = new(Network)
	rpcClient.Model.Nodes = map[NodeID]*Node{}
	rpcClient.Model.NetworkID = *net
	rpcClient.LocalInfo.NetworkID = *net

	fmt.Println(rpcendpoint)
	node, err := rpcClient.buildNode(rpcendpoint)
	if err != nil {
		return err
	}
	rpcClient.Model.Genesis = node.JsonInfo.Protocols.Eth.Genesis
	rpcClient.LocalInfo.Genesis = node.JsonInfo.Protocols.Eth.Genesis
	fmt.Println("Reachable Node!", node.ID)
	rpcClient.addNode(node)
	return nil
}

func (rpcClient *Client) buildNode(rpcendpoint string) (*Node, error) {
	version, err := rpcClient.getClientVersion(rpcendpoint)
	if err != nil {
		return nil, err
	}
	v := strings.Split(*version, "/")[0]
	switch v {
	case "besu":
		ni, err := rpcClient.GetNodeInfo(rpcendpoint)
		if err != nil {
			return nil, err
		}
		node := new(Node)
		tokens := strings.Split(rpcendpoint, ":")
		node.RPCPort, _ = strconv.Atoi(tokens[1])
		node.RPCURLs = map[string]bool{tokens[0]: true}
		FillNodeFromNodeInfo(node, ni)
		node.IsReachable = true
		node.LastSeen = time.Now()
		return node, nil

	default:
		return nil, fmt.Errorf("Not supported client:", v)
	}
	return nil, fmt.Errorf("You have reached unreachable code")
}

func (rpcClient *Client) getClientVersion(rpcendpoint string) (*string, error) {
	data := rpcClient.NewCallData("web3_clientVersion")
	data.Context.TargetRPCEndpoint = rpcendpoint
	v := new(string)
	err := rpcClient.actualRpcCall(data, v)
	return v, err
}

func (rpcClient *Client) findPeersOf(n *Node) error {
	data := rpcClient.NewCallData("admin_peers")
	data.Context.TargetRPCEndpoint = n.PrefRPCURL + ":" + strconv.Itoa(n.RPCPort)
	pi := new([]PeerInfo)
	err := rpcClient.actualRpcCall(data, pi)
	if err != nil {
		n.IsReachable = false
		return err
	}
	n.LastSeen = time.Now()

	n.Peers = map[NodeID]Peer{}

	for _, p := range *pi {
		peerNode := Peer{}
		peerNode.ID = NodeID(p.ID[2:])
		peerNode.PInfo = p
		n.Peers[peerNode.ID] = peerNode
	}

	return nil
}

func (rpcClient *Client) addNode(nd *Node) bool {
	if _, known := rpcClient.Model.Nodes[nd.ID]; known {
		return false
	}
	rpcClient.Model.Nodes[nd.ID] = nd

	nd.ticker = time.NewTicker(defaultProbeInterval)
	//TODO handle potential nil pointer
	wg, _ := rpcClient.runContext.Value("WaitGroup").(*sync.WaitGroup)
	wg.Add(1)
	go rpcClient.runNodeProbe(nd)

	return true

}

func (rpcClient *Client) runNodeProbe(nd *Node) {
	defer rpcClient.wg.Done()
	for {
		select {
		case <-rpcClient.runContext.Done():
			log.Println("Stopping probing on ", nd.ID.Short())
			return
		case <-nd.ticker.C:
			rpcClient.probe(nd)
		}
	}

}

var PortSearchScope = 5

func (rpcClient *Client) probe(nd *Node) {
	fmt.Println("Probing ", nd.ID)
	if nd.IsReachable {
		rpcClient.probeReachableNode(nd)
	} else {
		for url, _ := range nd.RPCURLs {
			var found = false
			//scan for rpc port
			for i := -PortSearchScope; i < PortSearchScope; i++ {
				port := nd.RPCPort + i
				rpcendpoint := url + ":" + strconv.Itoa(port)
				ni, e := rpcClient.GetNodeInfo(rpcendpoint)
				if e != nil || ni == nil {
					continue
				}
				if ni.ID == string(nd.ID) {
					FillNodeFromNodeInfo(nd, ni)
					nd.RPCPort = port
					nd.IsReachable = true
					nd.LastSeen = time.Now()
					nd.PrefRPCURL = url
					found = true
					break
				}
			}
			if found {
				break
			}
		}

	}

}

func (rpcClient *Client) probeReachableNode(nd *Node) {
	//Look if any new node in peers
	err := rpcClient.findPeersOf(nd)
	if err != nil {
		log.Println("Error on probing", nd.ID.Short(), err)
		nd.IsReachable = false
		nd.Peers = map[NodeID]Peer{}
		return
	}
	for id, p := range nd.Peers {
		ip := strings.Split(p.PInfo.Network.RemoteAddress, ":")[0]
		if rpcClient.addNode(NewUreachableNode(id, ip, nd.RPCPort)) {
			log.Println("New Node found:", id)
		}
	}
	err = rpcClient.findBlockNoOf(nd)
	if err != nil {
		log.Println("Error on probing", nd.ID.Short(), err)
	}
	err = rpcClient.TxPoolOf(nd)
	if err != nil {
		fmt.Println(err)
	}
	err = rpcClient.IsNodeSyncing(nd)
	if err != nil {
		fmt.Println(err)
	}

}

func (rpcClient *Client) TxPoolOf(nd *Node) error {
	data := rpcClient.NewCallData("txpool_besuTransactions")
	data.Context.TargetRPCEndpoint = nd.PrefRPCURL + ":" + strconv.Itoa(nd.RPCPort)
	txp := new([]TxpoolResult)
	err := rpcClient.actualRpcCall(data, txp)
	if err != nil {
		nd.IsReachable = false
		fmt.Println(err)
		return err
	}
	nd.LastSeen = time.Now()
	nd.Txpool = *txp
	return nil

}

func (rpcClient *Client) IsNodeSyncing(nd *Node) error {
	data := rpcClient.NewCallData("eth_syncing")
	data.Context.TargetRPCEndpoint = nd.PrefRPCURL + ":" + strconv.Itoa(nd.RPCPort)
	sync := new(bool)
	err := rpcClient.actualRpcCall(data, sync)
	if err != nil {
		nd.IsReachable = false
		fmt.Println(err)
		return err
	}
	nd.LastSeen = time.Now()
	nd.Syncing = *sync
	return nil

}

func NewUreachableNode(id NodeID, rpcip string, port int) *Node {
	n := new(Node)
	n.ID = id
	n.RPCPort = port
	n.RPCURLs = map[string]bool{rpcip: true}
	n.IsReachable = false
	return n
}

func (rpcClient *Client) findBlockNoOf(n *Node) error {
	data := rpcClient.NewCallData("eth_blockNumber")
	data.Context.TargetRPCEndpoint = n.PrefRPCURL + ":" + strconv.Itoa(n.RPCPort)
	bnp := ""
	err := rpcClient.actualRpcCall(data, &bnp)
	if err != nil {
		return err
	}
	n.LastSeen = time.Now()
	n.BlockNumber, err = strconv.ParseInt(bnp[2:], 16, 32)
	return err
}

func (rpcClient *Client) NodeActions(data *templates.RenderData, rq *http.Request) {
	data.TemplateName = "nodepage"
	nodeid := rq.Form.Get(keyword_nodeid)
	fmt.Println(nodeid)
	node, ok := rpcClient.Model.Nodes[NodeID(nodeid)]
	if !ok {
		data.Error = fmt.Errorf("No such node: " + nodeid)
		return
	}
	action := rq.Form.Get(keyword_action)

	data.BodyData = node
	switch action {
	case "addaddress":
		addr := rq.Form.Get("value")
		if len(addr) > 0 {
			node.RPCURLs[addr] = true
		}
	case "removeaddress":
		addr := rq.Form.Get("value")
		delete(node.RPCURLs, addr)
	case "setport":
		port := rq.Form.Get("value")
		p, err := strconv.Atoi(port)
		if err != nil {
			data.Error = err
			return
		}
		node.RPCPort = p
	default:

	}
}

var scanrange = 600

func (rpcClient *Client) BlockActions(data *templates.RenderData, rq *http.Request) {
	data.TemplateName = "blockpage"
	blockid := rq.Form.Get(keyword_blocknum)
	var blocknum int64
	var err error
	if len(blockid) > 0 {
		blocknum, err = strconv.ParseInt(blockid, 10, 32)
		if err != nil {
			data.Error = err
			return
		}
	} else {
		blockhex := rq.Form.Get("blockhex")
		if len(blockhex) < 3 {
			data.Error = fmt.Errorf("Wrong hex block number")
			return
		}
		blocknum, err = strconv.ParseInt(blockhex[2:], 16, 32)
		if err != nil {
			data.Error = err
			return
		}
	}

	action := rq.Form.Get(keyword_action)
	var scan = false
	if action=="scan_forward" || action=="scan_back" {
		scan = true
	}

	var next = true
	var block *BlockResult
	for i:=0; next; i++ {
		var delta = int64(0)

		switch action {
		case "forward", "scan_forward":
			delta = 1
		case "back", "scan_back":
			delta = -1
		}
		blocknum += delta
		fmt.Println("Fetching block No", blocknum)
		//`{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x68B3", true],"id":1}' http://localhost:8546`
		blockhex := fmt.Sprintf("0x%x", blocknum)

		calldat := rpcClient.NewCallData("eth_getBlockByNumber")
		calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
		calldat.Command.Params = []interface{}{blockhex, true}
		block = new(BlockResult)
		err = rpcClient.actualRpcCall(calldat, block)
		if err != nil {
			data.Error = err
			return
		}
		next = false
		if scan && len(block.Transactions) == 0 && i < scanrange {

				next = true

		}
	}
	data.BodyData = block
}
