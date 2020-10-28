package rpcclient

import (
	"encoding/json"
	"time"
)

type EthResponse struct {
	ID      int             `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *EthError       `json:"error"`
}

// EthError - ethereum error
type EthError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type EthCommand struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      uint          `json:"id"`
}

type NodeInfo struct {
	Enode      string `json:"enode"`
	ListenAddr string `json:"listenAddr"`
	IP         string `json:"ip"`
	Name       string `json:"name"`
	ID         string `json:"id"`
	Ports      struct {
		Discovery int `json:"discovery"`
		Listener  int `json:"listener"`
	} `json:"ports"`
	Protocols struct {
		Eth struct {
			Config struct {
				ChainID         int `json:"chainId"`
				PetersburgBlock int `json:"petersburgBlock"`
				Ibft2           struct {
					EpochLength           int `json:"epochLength"`
					BlockPeriodSeconds    int `json:"blockPeriodSeconds"`
					RequestTimeoutSeconds int `json:"requestTimeoutSeconds"`
				} `json:"ibft2"`
			} `json:"config"`
			Difficulty int    `json:"difficulty"`
			Genesis    string `json:"genesis"`
			Head       string `json:"head"`
			Network    int    `json:"network"`
		} `json:"eth"`
	} `json:"protocols"`
}

type PeerInfo struct {
	Version string   `json:"version"`
	Name    string   `json:"name"`
	Caps    []string `json:"caps"`
	Network struct {
		LocalAddress  string `json:"localAddress"`
		RemoteAddress string `json:"remoteAddress"`
	} `json:"network"`
	Port string `json:"port"`
	ID   string `json:"id"`
}

type TxpoolResult []struct {
	Hash                      string    `json:"hash"`
	IsReceivedFromLocalSource bool      `json:"isReceivedFromLocalSource"`
	AddedToPoolAt             time.Time `json:"addedToPoolAt"`
}
