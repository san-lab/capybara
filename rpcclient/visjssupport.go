package rpcclient

import (
	"encoding/json"
	"html/template"
	"log"
	"sort"
	"strconv"

	"fmt"
	"strings"
)

func JSEscape(vn interface{}) template.JS {
	ret, err := json.Marshal(vn)
	if err != nil {
		log.Println(err)
	}
	return template.JS(ret)
}

func (bcn *Network) VisjsNodes() []Visnode {
	var vn []Visnode

	//We sort the keys to have a deterministic order
	var keys []string
	for id := range bcn.Nodes {
		keys = append(keys, string(id))
	}
	sort.Strings(keys)
	for _, key := range keys {
		nd := bcn.Nodes[NodeID(key)]
		ports := strconv.Itoa(nd.RPCPort)

		label := "Node At: " + nd.PrefRPCURL + ":" + ports
		label += "\nID:" + nd.ID.Short()
		//title := "ID: " + string(nd.ID) + "<br/>"
		title := "Current block: " + strconv.Itoa(int(nd.BlockNumber))
		title += "<br/> Last seen: " + nd.FormattedLastSeen()
		title += "<br/> Trailing?: " + fmt.Sprint(bcn.IsTrailing(nd))

		//txp, err := json.Marshal(nd.Txpool)
		//if err == nil {
		//	title += "<br/>TxPool: " + string(txp)
		//} else {
		//	title += "<br/>TxPool: " + fmt.Sprint(err)
		//}
		title += "<br/>TxPool: [" + strconv.Itoa(len(nd.Txpool)) +"]"

		title += "<br/>Syncing: " + fmt.Sprint(nd.Syncing)

		vi := Visnode{Id: nd.ID, Label: label, Title: title, Image: "/static/ethereum_32x32.png", Shape: "image", Font: Font{Align: "left"}}
		if nd.Syncing {
			vi.Image = "/static/ethereum-full_32x32_syncing.png"
		} else if nd.IsReachable {
			vi.Image = "/static/ethereum-full_32x32.png"
		}

		vn = append(vn, vi)
	}
	return vn
}

func (bcn *Network) VisjsEdges() []Visedge {
	var ve []Visedge
	for _, nd := range bcn.Nodes {
		if !nd.IsReachable {
			continue
		}
		for _, pnd := range nd.Peers {

			ve = append(ve, VisjsEdge(nd.ID, "", pnd.ID))

		}

	}

	return ve
}

func (net *Network) VisjsData() Visdata {
	vd := Visdata{}
	vd.Nodes = net.VisjsNodes()
	vd.Edges = net.VisjsEdges()
	return vd

}

func VisjsEdge(base NodeID, addr string, peer NodeID) Visedge {
	e := Visedge{ID: string(base) + "TO" + string(peer), From: base, To: peer, Label: addr}
	e.Color.Color = "blue"
	e.Color.Highlight = "blue"
	e.Color.Hover = "green"
	//e.Arrows = "moving-dot"
	e.Smooth = strings.Compare(string(base), string(peer)) > 0
	f := Font{Size: 1, Align: "bottom"}
	e.Font = f
	e.BorderWidth = 1
	e.ShapeProperties = ShapeProperties{true}
	return e
}

type Visnode struct {
	Id    NodeID `json:"id,omitempty"`
	Label string `json:"label"`
	Image string `json:"image"`
	Shape string `json:"shape"`
	Color Color  `json:"color,omitempty"`
	Font  Font   `json:"font,omitempty"`
	Title string `json:"title,omitempty"`
}

type Visedge struct {
	ID              string          `json:"id,omitempty"`
	From            NodeID          `json:"from"`
	To              NodeID          `json:"to"`
	Arrows          string          `json:"arrows,omitempty"`
	Label           string          `json:"label"`
	Font            Font            `json:"font,omitempty"`
	Color           Color           `json:"color,omitempty"`
	Smooth          bool            `json:"smooth"`
	Length          int             `json:"length,omitempty"`
	BorderWidth     int             `json:"borderWidth,omitempty"`
	ShapeProperties ShapeProperties `json:"shapeProperties,omitempty"`
}

type ShapeProperties struct {
	UseBorderWithImage bool `json:"useBorderWithImage,omitempty"`
}

type Color struct {
	Color     string `json:"color"`
	Highlight string `json:"highlight,omitempty"`
	Hover     string `json:"hover,omitempty"`
}

type Font struct {
	Size  int    `json:"size,omitempty"`
	Align string `json:"align,omitempty"`
}

type Visdata struct {
	Nodes      []Visnode `json:"nodes"`
	Edges      []Visedge `json:"edges"`
	NodesTable string    `json:"nodestable"`
}
