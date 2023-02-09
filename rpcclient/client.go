package rpcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

//A rest api client, wrapping an http client
//The struct also contains a map of addresses of known nodes' end-points
//The field Port - to memorize the default Port (a bit of a stretch)
type Client struct {
	DefaultRPCEndpoint string // host:port
	DefRPCTLS          bool
	UserAgent          string
	httpClient         HttpClient
	seq                uint
	LocalInfo          CallContext
	//NetModel             BlockchainNet
	DebugMode bool
	//UnreachableAddresses map[string]MyTime
	MockMode         bool
	DumpRPC          bool
	blockedAddresses map[string]bool
	Model            *Network
	runContext       context.Context
	wg               *sync.WaitGroup
}

type HttpClient interface {
	Do(r *http.Request) (*http.Response, error)
}

const defaultTimeout = 3 * time.Second

type ClientConfig struct {
	MockMode bool
	DumpRPC  bool
	URL      string
	TLS      bool
}

//Creates a new rest api client
//If something like ("www.node:8666",8545) is passed, an error is thrown
func NewClient(cc ClientConfig, ctx context.Context) (c *Client, err error) {
	c = &Client{}
	c.MockMode = cc.MockMode
	c.DumpRPC = cc.DumpRPC
	c.DefaultRPCEndpoint = cc.URL
	c.DefRPCTLS = cc.TLS

	c.httpClient = http.DefaultClient
	c.httpClient.(*http.Client).Timeout = defaultTimeout

	c.seq = 0
	//TODO handle error
	c.LocalInfo, _ = GetLocalInfo()
	//c.NetModel = *NewBlockchainNet()
	//c.UnreachableAddresses = map[string]MyTime{}
	//TODO handle potential nil pointer
	c.wg, _ = ctx.Value("WaitGroup").(*sync.WaitGroup)
	c.blockedAddresses = map[string]bool{}
	c.runContext = ctx
	go c.deferSavingConfig()
	c.initModel()
	log.Println("Init done")
	return
}

//A wrapper to pass around the call Context, RPC, Result, and Error (if any)
type CallData struct {
	Context      CallContext
	Command      EthCommand
	Response     EthResponse
	Parsed       bool // if the "result" has been successfully decoded to a specific structure
	ParsedResult interface{}
	NodeAddress  string
	NodeRPCport  string
	RawJson      bool //How to parse
	JsonRequest  string
	JsonResponse string
}

//TODO: expand this stub
type CallContext struct {
	ClientHostName    string
	ClientIp          string
	TargetRPCEndpoint string
	RawMode           bool
	RequestPath       string
	Refresh           int
	Watchdog          bool
	WatchdogInterval  int64
	NetworkID         string
	Genesis           string
}

//Implementing the HeaderData methods
func (cc CallContext) GetRefresh() int {
	return cc.Refresh
}
func (cc *CallContext) SetRefresh(i int) {
	cc.Refresh = i
}

//The name says it all
func (rpcClient *Client) SetTimeout(timeout time.Duration) {
	//if !rpcClient.MockMode {
	rpcClient.httpClient.(*http.Client).Timeout = defaultTimeout
	//}
}

//Just a sequence to number the rest calls (the "id" field)
//TODO: wrap the sequence as a in a Type
func (rpcClient *Client) nextID() (id uint) {
	id = rpcClient.seq
	rpcClient.seq++
	return
}

var hasHttpPrefix = regexp.MustCompile(`^https?:\/\/`)
var tlsPrefix = `https://`
var plainPrefix = `http://`

//Generic call to the ethereum api's.
func (rpcClient *Client) ActualRpcCall(data *CallData, result interface{}) error {
	if rpcClient.blockedAddresses[data.Context.TargetRPCEndpoint] {
		return errors.New("Blocked address:" + data.Context.TargetRPCEndpoint)
	}
	data.Command.Id = rpcClient.nextID()
	jcom, _ := json.Marshal(data.Command)
	if rpcClient.DebugMode {
		log.Println(string(jcom))
	}
	var host = data.Context.TargetRPCEndpoint
	p := hasHttpPrefix.Find([]byte(host))

	if len(p) == 0 {
		if rpcClient.DefRPCTLS {
			host = tlsPrefix + host
		} else {
			host = plainPrefix + host
		}
	}

	req, err := http.NewRequest("POST", host, bytes.NewReader(jcom))
	if err != nil {
		rpcClient.log(fmt.Sprintf("%s", err))
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", rpcClient.UserAgent)
	req.Header.Set("Content-type", "application/json")
	resp, err := rpcClient.httpClient.Do(req)
	if err != nil {
		//rpcClient.NetModel.UnreachableNodes[GhostNode(host)] = MyTime(time.Now())
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err = errors.New(resp.Status)
		return err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if rpcClient.DebugMode {
		log.Println(string(respBytes))
	}
	if err != nil {
		rpcClient.log(fmt.Sprintf("%s", err))
		return err
	}

	if rpcClient.DumpRPC {
		key, _, _ := net.SplitHostPort(req.URL.Host)
		key = key + "_" + data.Command.Method + ".json"
		log.Println("dumping " + key)
		ioutil.WriteFile(key, respBytes, 0644)
	}

	data.JsonRequest = string(jcom)
	var buf bytes.Buffer
	err = json.Indent(&buf, respBytes, "", " ")
	if err != nil {
		rpcClient.log(fmt.Sprint(err))
	} //irrelevant error not worth returning
	data.JsonResponse = buf.String()
	rpcClient.log("Returned:\n" + fmt.Sprintf("%s", resp.Header))
	rpcClient.log("Returned:\n" + data.JsonResponse)
	if err != nil {
		rpcClient.log(fmt.Sprint(err))
	}
	err = json.Unmarshal(respBytes, &data.Response)
	if err != nil {
		return err
	}
	if data.Response.Error != nil {
		err = fmt.Errorf("RPC Error: %v, %s", data.Response.Error.Code, data.Response.Error.Message)
		return err
	}

	// now try to parse the result into the interface provided
	err = json.Unmarshal(data.Response.Result, result)
	return err
}

//The name says it all.
// "method" name is needed for constructing the RPC field
//     - which is complete and only the "ID" integer is meant to be changed
func (rpcClient *Client) NewCallData(method string) *CallData {
	com := EthCommand{"2.0", method, []interface{}{}, 0}
	ctx := rpcClient.LocalInfo // Cloning. This at least is my intention ;-)
	calldata := &CallData{Context: ctx, Command: com, Response: EthResponse{}}
	return calldata
}

//Just a stub of a function gathering host system info
func GetLocalInfo() (CallContext, error) {
	hostname, err := os.Hostname()
	conn, err := net.Dial("udp", "8.8.8.8:80")
	var ipaddress string
	if err != nil {
		ipaddress = ""
		log.Println("No network")
	} else {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		ipaddress = localAddr.IP.String()
	}
	return CallContext{ClientHostName: hostname, ClientIp: ipaddress}, err
}

func (rpcClient *Client) log(s string) {
	if rpcClient.DebugMode {
		log.Println(s)
	}
}

func (rpcClient *Client) PrintModel(w io.Writer) {
	b, e := json.MarshalIndent(rpcClient.Model, " ", " ")
	if e != nil {
		fmt.Fprintln(w, e)
	} else {
		fmt.Fprintln(w, string(b))
	}
}
