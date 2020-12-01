package httphandler

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"bytes"
	"github.com/san-lab/capybara/rpcclient"
	"github.com/san-lab/capybara/templates"
	"strconv"
)

const passwdFile = "http.passwd.json"

//This is the glue between the http requests and the (hopefully) generic RPC client

type LilHttpHandler struct {
	defaultContext rpcclient.CallContext
	config         Config
	rpcClient      *rpcclient.Client
	runContext     context.Context
	renderer       *templates.Renderer
	refresh        int
}

//Creating a naw http handler with its embedded rpc client and html renderer
func NewHttpHandler(c Config, ctx context.Context) (lhh *LilHttpHandler, err error) {
	lhh = &LilHttpHandler{}
	lhh.config = c
	lhh.runContext = ctx
	lhh.renderer = templates.NewRenderer()

	lhh.rpcClient, err = rpcclient.NewClient(c.RPCFirstEntry, c.MockMode, c.DumpRPC, ctx)
	lhh.rpcClient.DebugMode = c.DebugMode

	if c.BasicAuth {
		lhh.initPasswords(ctx)
	}
	lhh.refresh = 5
	return lhh, err
}

const keyword_init = "initialize"
const keyword_templates = "loadtemplates"
const keyword_setrefresh = "setrefresh"
const keyword_rpc = "rpc"
const arg_refreshrate = "rate"
const keyword_node = "node"
const keyword_block = "block"
const keyword_tx = "transaction"
const keyword_txpool = "txpool"
const keyword_txpoolTx = "txpoolTx"

// Handles incoming requests. Some will be forwarded to the RPC client.
// Assumes the request path has either: 1 part - interpreted as a /command with logic implemented within the client
//                                  or: 2 parts - interpreted as /node/ethMethod
// The port No set at Client initialization is used for the RPC call
func (lhh *LilHttpHandler) Handler(w http.ResponseWriter, r *http.Request) {

	isSlash := func(c rune) bool { return c == '/' }
	f := strings.FieldsFunc(r.URL.Path, isSlash)
	//log.Println(f)
	rdata := templates.RenderData{HeaderData: &lhh.rpcClient.LocalInfo, TemplateName: templates.Home, Client: lhh.rpcClient}
	rdata.BodyData = lhh.rpcClient.Model
	rdata.HeaderData.SetRefresh(lhh.refresh)
	r.ParseForm()

	if len(f) > 0 { // Some URI path provided
		switch f[0] {
		case keyword_templates:
			lhh.renderer.LoadTemplates()

		case keyword_init:
			e := lhh.rpcClient.Initialize()
			rdata.Error = e
		case keyword_rpc:
			lhh.rpcClient.DirectMethod(w, r)
			return
		case keyword_node:
			lhh.rpcClient.NodeActions(&rdata, r)
		case keyword_tx:
			lhh.rpcClient.Transactions(&rdata, r)
		case keyword_block:
			lhh.rpcClient.BlockActions(&rdata, r)
		case keyword_txpool:
			lhh.rpcClient.TxPool(&rdata, r)
		case keyword_txpoolTx:
			lhh.rpcClient.TxPoolTx(&rdata, r)
		case "printnetwork":
			lhh.rpcClient.PrintModel(w)
		case keyword_setrefresh:
			rs := r.Form.Get(arg_refreshrate)
			ri, e := strconv.Atoi(rs)
			if e == nil {
				lhh.refresh = ri
				rdata.HeaderData.SetRefresh(ri)
			} else {
				rdata.Error = e
			}
		case "visnodes":
			w.Header().Add("Content-type", "application/json")
			fmt.Fprintln(w, rpcclient.JSEscape(lhh.rpcClient.Model.VisjsNodes()))
			return
		case "visedges":
			w.Header().Add("Content-type", "application/json")
			fmt.Fprintln(w, rpcclient.JSEscape(lhh.rpcClient.Model.VisjsEdges()))
			return
		case "visnet":
			w.Header().Add("Content-type", "application/json")
			vd := lhh.rpcClient.Model.VisjsData()
			vd.NodesTable = lhh.GetNodesTable()

			fmt.Fprintln(w, rpcclient.JSEscape(vd))
			return
		case "nodestable":
			fmt.Fprintln(w, lhh.GetNodesTable())
			return

		default:

			rdata.HeaderData.SetRefresh(5)
			rdata.BodyData = fmt.Sprintln(f[0] + " called")
			lhh.renderer.RenderResponse(w, &rdata)
			return
		}
	}

	lhh.renderer.RenderResponse(w, &rdata)

}

type handler func(w http.ResponseWriter, r *http.Request)

func (lhh *LilHttpHandler) GetHandler(withAuth bool) handler {
	if withAuth {
		return lhh.BasicAuthHandler
	}
	return lhh.Handler
}

type Config struct {
	RPCFirstEntry string
	MockMode      bool
	DumpRPC       bool
	StartWatchdog bool
	BasicAuth     bool
	DebugMode     bool
}

func (lhh *LilHttpHandler) GetNodesTable() string {
	s := ""
	b := bytes.NewBufferString(s)
	lhh.renderer.Templates.ExecuteTemplate(b, "nodestable", lhh.rpcClient.Model)
	return b.String()
}
