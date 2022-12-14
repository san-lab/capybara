package rpcclient

import (
	"encoding/json"
	"strconv"
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

type BlockResult struct {
	Number           string              `json:"number"`
	Hash             string              `json:"hash"`
	MixHash          string              `json:"mixHash"`
	ParentHash       string              `json:"parentHash"`
	Nonce            string              `json:"nonce"`
	Sha3Uncles       string              `json:"sha3Uncles"`
	LogsBloom        string              `json:"logsBloom"`
	TransactionsRoot string              `json:"transactionsRoot"`
	StateRoot        string              `json:"stateRoot"`
	ReceiptsRoot     string              `json:"receiptsRoot"`
	Miner            string              `json:"miner"`
	Difficulty       string              `json:"difficulty"`
	TotalDifficulty  string              `json:"totalDifficulty"`
	ExtraData        string              `json:"extraData"`
	Size             string              `json:"size"`
	GasLimit         string              `json:"gasLimit"`
	GasUsed          string              `json:"gasUsed"`
	Timestamp        string              `json:"timestamp"`
	Uncles           []interface{}       `json:"uncles"`
	Transactions     []TransactionResult `json:"transactions"`
}

type TransactionResult struct {
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Hash             string `json:"hash"`
	Input            string `json:"input"`
	Nonce            string `json:"nonce"`
	To               string `json:"to"`
	TransactionIndex string `json:"transactionIndex"`
	Value            string `json:"value"`
	V                string `json:"v"`
	R                string `json:"r"`
	S                string `json:"s"`
}

type TxH struct {
	Tx *TransactionResult
	Tr *TransactionReceipt
	Pr string //proof
}

func (th *TxH) GetReceipt(rpcClient *Client) error {
	if th.Tx != nil {
		calldat := rpcClient.NewCallData("eth_getTransactionReceipt")
		calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
		calldat.Command.Params = []interface{}{th.Tx.Hash}
		trRec := new(TransactionReceipt)
		err := rpcClient.actualRpcCall(calldat, trRec)
		if err != nil {

			return err
		}
		th.Tr = trRec
	}
	return nil
}

type TransactionReceipt struct {
	BlockHash         string        `json:"blockHash"`
	BlockNumber       string        `json:"blockNumber"`
	ContractAddress   string        `json:"contractAddress"`
	CumulativeGasUsed string        `json:"cumulativeGasUsed"`
	From              string        `json:"from"`
	GasUsed           string        `json:"gasUsed"`
	EffectiveGasPrice string        `json:"effectiveGasPrice"`
	Logs              []interface{} `json:"logs"`
	LogsBloom         string        `json:"logsBloom"`
	Status            string        `json:"status"`
	To                interface{}   `json:"to"`
	TransactionHash   string        `json:"transactionHash"`
	TransactionIndex  string        `json:"transactionIndex"`
}

type TxPoolResult struct {
	Transactions []TxpoolTransaction
}

type TxpoolTransaction struct {
	Hash                      string    `json:"hash"`
	IsReceivedFromLocalSource bool      `json:"isReceivedFromLocalSource"`
	AddedToPoolAt             time.Time `json:"addedToPoolAt"`
}

type TxPoolTransactionList struct {
	Transactions []TxPoolTransactionResult
}

type TxPoolTransactionResult struct {
	From     string `json:"from"`
	Gas      string `json:"gas"`
	GasPrice string `json:"gasPrice"`
	Hash     string `json:"hash"`
	Input    string `json:"input"`
	Nonce    string `json:"nonce"`
	To       string `json:"to"`
	Value    string `json:"value"`
	V        string `json:"v"`
	R        string `json:"r"`
	S        string `json:"s"`
}

func (block *BlockResult) TimestampToTime() time.Time {
	t, e := strconv.ParseInt(block.Timestamp[2:], 16, 64)
	if e != nil {
		return time.Time{}
	}
	return time.Unix(t, 0)

}

func (block *BlockResult) BlockNumInt64() int64 {
	blocknum, err := strconv.ParseInt(block.Number[2:], 16, 32)
	if err != nil {
		return 0
	}
	return blocknum
}
