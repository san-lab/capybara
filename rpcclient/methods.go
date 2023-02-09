package rpcclient

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"io/ioutil"
	"regexp"
	"sort"

	"github.com/san-lab/capybara/templates"
)

const keyword_action = "action"
const keyword_nodeid = "nodeid"
const keyword_blocknum = "blocknum"

func (rpcClient *Client) GetNetwork(rpcendpoint string) (*string, error) {
	data := rpcClient.NewCallData("net_version")
	data.Context.TargetRPCEndpoint = rpcendpoint

	net := new(string)
	err := rpcClient.ActualRpcCall(data, net)
	if err != nil {
		return nil, err
	}
	return net, nil
}

func (rpcClient *Client) GetNodeInfo(rpcendpoint string) (*NodeInfo, error) {
	data := rpcClient.NewCallData("admin_nodeInfo")
	data.Context.TargetRPCEndpoint = rpcendpoint
	ni := new(NodeInfo)
	err := rpcClient.ActualRpcCall(data, ni)
	return ni, err
}

func (rpcClient *Client) DirectMethod(w http.ResponseWriter, rq *http.Request) (err error) {
	fmt.Println("Direct method call")
	rq.ParseForm()

	meth := rq.Form.Get("method")
	if len(meth) > 0 {
		data := rpcClient.NewCallData(meth)
		//"parN"=parameterValue
		paramValidator := regexp.MustCompile(`par\d$`)
		var keys []string
		for k := range rq.Form {
			if paramValidator.MatchString(k) {
				keys = append(keys, k)

			}
		}
		if len(keys) > 0 {
			sort.Strings(keys)
			for _, pk := range keys {
				data.Command.Params = append(data.Command.Params, rq.Form[pk][0])
			}
		}
		// End of param handling
		data.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
		err := rpcClient.ActualRpcCall(data, new(string))
		fmt.Fprintln(w, "Error: ", err)
		fmt.Fprintln(w, string(data.Response.Result))
	} else {
		fmt.Fprintln(w, "No method supplied")
	}
	return err
}

func (rpcClient *Client) Initialize() error {
	rpcendpoint := rpcClient.DefaultRPCEndpoint
	fmt.Println("initializing")
	net, err := rpcClient.GetNetwork(rpcendpoint)
	if err != nil {
		fmt.Println(err)
		return err
	}
	rpcClient.Model.NetworkID = *net
	rpcClient.LocalInfo.NetworkID = *net

	if rpcClient.Model == nil {
		rpcClient.Model = new(Network)
	}

	if rpcClient.Model.Nodes == nil || len(rpcClient.Model.Nodes) == 0 {
		rpcClient.Model.Nodes = map[NodeID]*Node{}
		fmt.Println(rpcendpoint)
		node, err := rpcClient.buildNode(rpcendpoint)
		if err != nil {
			return err
		}
		rpcClient.Model.Genesis = node.JsonInfo.Protocols.Eth.Genesis
		rpcClient.LocalInfo.Genesis = node.JsonInfo.Protocols.Eth.Genesis
		fmt.Println("Reachable Node!", node.ID)
		rpcClient.addNode(node)

	} else {
		for _, node := range rpcClient.Model.Nodes {
			if !node.probed {
				go rpcClient.runNodeProbe(node)
			}

		}
	}

	return nil
}

var besu = regexp.MustCompile("besu")
var edge = regexp.MustCompile("edge")

func (rpcClient *Client) buildNode(rpcendpoint string) (*Node, error) {
	version, err := rpcClient.getClientVersion(rpcendpoint)
	if err != nil {
		return nil, err
	}

	node := new(Node)
	if len(besu.FindString(*version)) > 0 {
		ni, err := rpcClient.GetNodeInfo(rpcendpoint)
		if err != nil {
			return nil, err
		}

		tokens := strings.Split(rpcendpoint, ":")
		node.RPCPort, _ = strconv.Atoi(tokens[1])
		node.RPCURLs = map[string]bool{tokens[0]: true}
		FillNodeFromNodeInfo(node, ni)
		node.IsReachable = true
		node.LastSeen = time.Now()
		node.Client = "besu"
		return node, nil
	}

	if len(edge.FindString(*version)) > 0 {
		node.Client = "edge"
		node.IsReachable = true
		node.RPCURLs = map[string]bool{rpcendpoint: true}
		node.RPCPort = 443
		return node, nil
	}

	return nil, fmt.Errorf("Not supported client: %v", *version)

}

func (rpcClient *Client) getClientVersion(rpcendpoint string) (*string, error) {
	data := rpcClient.NewCallData("web3_clientVersion")
	data.Context.TargetRPCEndpoint = rpcendpoint
	v := new(string)
	err := rpcClient.ActualRpcCall(data, v)
	return v, err
}

func (rpcClient *Client) findPeersOf(n *Node) error {
	data := rpcClient.NewCallData("admin_peers")
	data.Context.TargetRPCEndpoint = n.PrefRPCURL + ":" + strconv.Itoa(n.RPCPort)
	pi := new([]PeerInfo)
	err := rpcClient.ActualRpcCall(data, pi)
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
		//log.Println("Not adding a known node:", nd.ID)
		return false
	}
	rpcClient.Model.Nodes[nd.ID] = nd
	go rpcClient.runNodeProbe(nd)

	return true

}

var networkfilename = "networkjson.json"

func (rpcClient *Client) initModel() {
	defer rpcClient.Initialize()
	model := new(Network)
	rpcClient.Model = model
	b, e := ioutil.ReadFile(networkfilename)
	if e != nil {
		log.Println(e)
		return
	}
	e = json.Unmarshal(b, model)
	if e != nil {
		log.Println(e)
		return
	}

}

func (rpcClient *Client) deferSavingConfig() {
	rpcClient.wg.Add(1)
	defer rpcClient.wg.Done()
	for {
		select {
		case <-rpcClient.runContext.Done():
			b, err := json.Marshal(rpcClient.Model)
			if err != nil {
				log.Println(err)
				return
			}
			log.Println("Saving net")
			ioutil.WriteFile(networkfilename, b, 0644)
			return
		}
	}
}

func (rpcClient *Client) runNodeProbe(nd *Node) {
	defer rpcClient.wg.Done()
	rpcClient.wg.Add(1)
	nd.ticker = time.NewTicker(defaultProbeInterval)
	nd.probed = true
	log.Println("Starting probing on ", nd.ID.Short())
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
	err := rpcClient.findBlockNoOf(nd)
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

	//Look if any new node in peers
	err = rpcClient.findPeersOf(nd)
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

}

func (rpcClient *Client) TxPoolOf(nd *Node) error {
	var method string
	//TODO finalize client names
	switch nd.Client {
	case "besu":
		method = "txpool_besuTransactions"
	case "edge":
		method = "txpool_content"
	default:

		return fmt.Errorf("unsupported client")
	}

	data := rpcClient.NewCallData(method)
	data.Context.TargetRPCEndpoint = nd.PrefRPCURL + ":" + strconv.Itoa(nd.RPCPort)
	txp := new([]TxpoolTransaction)
	err := rpcClient.ActualRpcCall(data, txp)
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
	err := rpcClient.ActualRpcCall(data, sync)
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
	err := rpcClient.ActualRpcCall(data, &bnp)
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
	action := rq.Form.Get(keyword_action)
	var blocknum int64
	var err error
	if len(blockid) > 0 {
		blocknum, err = strconv.ParseInt(blockid, 10, 32)
		if err != nil {
			data.Error = err
			return
		}
	} else if action == "find_tx" {
		rpcClient.Transactions(data, rq)
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

	var scan = false
	if action == "scan_forward" || action == "scan_back" {
		scan = true
	}

	var next = true
	var block *BlockResult
	for i := 0; next; i++ {
		var delta = int64(0)

		switch action {
		case "next", "scan_forward":
			delta = 1
		case "prev", "scan_back":
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
		err = rpcClient.ActualRpcCall(calldat, block)
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

func (rpcClient *Client) Transactions(data *templates.RenderData, rq *http.Request) {
	data.TemplateName = "txpage"

	var err error

	action := rq.Form.Get(keyword_action)
	ionproof := rq.FormValue("ionproof")
	blockhex := rq.Form.Get("blockhex")
	if action == "find_tx" {
		txHash := rq.Form.Get("tx_hash")
		if len(txHash) != 66 {
			data.Error = fmt.Errorf("Wrong tx hash")
			return
		}
		calldat := rpcClient.NewCallData("eth_getTransactionByHash")
		calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
		calldat.Command.Params = []interface{}{txHash}
		th := new(TxH)
		transaction := new(TransactionResult)
		err := rpcClient.ActualRpcCall(calldat, transaction)
		if err != nil {
			data.Error = err
			return
		}
		th.Tx = transaction
		th.GetReceipt(rpcClient)
		if ionproof == "on" {
			GetProof(rpcClient, th)
		}
		data.BodyData = th
		return
	} else if len(blockhex) > 0 {
		if len(blockhex) < 3 {
			data.Error = fmt.Errorf("Wrong hex block number")
			return
		}
		calldat := rpcClient.NewCallData("eth_getBlockByNumber")
		calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
		calldat.Command.Params = []interface{}{blockhex, true}
		block := new(BlockResult)
		err = rpcClient.ActualRpcCall(calldat, block)
		if err != nil {
			data.Error = err
			return
		}
		txindex := rq.Form.Get("txindex")
		i, e := strconv.Atoi(txindex)
		if e == nil {

			th := new(TxH)
			th.Tx = &(block.Transactions[i])
			th.GetReceipt(rpcClient)
			data.BodyData = th

			return
		} else {
			data.Error = fmt.Errorf("Wrong blockNumber and txIndex")
			return
		}
	} else {
		data.Error = fmt.Errorf("Unknown parameters")
		return
	}
}

func (rpcClient *Client) TxPool(data *templates.RenderData, rq *http.Request) {
	data.TemplateName = "txpoolpage"
	calldat := rpcClient.NewCallData("txpool_besuTransactions")
	calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
	calldat.Command.Params = []interface{}{}
	var txPoolList []TxpoolTransaction
	err := rpcClient.ActualRpcCall(calldat, &txPoolList)
	txpool := new(TxPoolResult)
	txpool.Transactions = txPoolList
	if err != nil {
		data.Error = err
		return
	}
	data.BodyData = txpool
	return
}

type filterParams struct {
	From struct {
		Eq string `json:"eq,omitempty"`
	} `json:"from,omitempty"`
	To struct {
		Eq string `json:"eq,omitempty"`
	} `json:"to,omitempty"`
	Nonce struct {
		Gt string `json:"gt,omitempty"`
	} `json:"nonce,omitempty"`
}

func (rpcClient *Client) TxPoolTx(data *templates.RenderData, rq *http.Request) {
	data.TemplateName = "txpoolTx"

	txFrom := rq.Form.Get("from")
	if len(txFrom) != 42 && txFrom != "from" {
		data.Error = fmt.Errorf("Invalid address for from")
		return
	}
	txTo := rq.Form.Get("to")
	if len(txTo) != 42 && txTo != "to" {
		data.Error = fmt.Errorf("Invalid address for to")
		return
	}
	txNonce := rq.Form.Get("nonce")

	calldat := rpcClient.NewCallData("txpool_besuPendingTransactions")
	calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
	filterParams := new(filterParams)
	if txFrom != "from" {
		filterParams.From.Eq = txFrom
	}
	if txTo != "to" {
		filterParams.To.Eq = txTo
	}
	if txNonce != "min nonce" {
		filterParams.Nonce.Gt = txNonce
	}
	calldat.Command.Params = []interface{}{1000, filterParams}
	var txPoolTxList []TxPoolTransactionResult
	err := rpcClient.ActualRpcCall(calldat, &txPoolTxList)
	txpoolTransactionList := new(TxPoolTransactionList)
	txpoolTransactionList.Transactions = txPoolTxList
	if err != nil {
		data.Error = err
		return
	}
	data.BodyData = txpoolTransactionList
	return
}
