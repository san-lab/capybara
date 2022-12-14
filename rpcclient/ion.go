//xgo:build ion
// +xbuild ion

package rpcclient

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"

	"github.com/clearmatics/ion-cli/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/san-lab/capybara/templates"
)

func GetIonProof(rpcClient *Client, txHash string) ([]byte, error) {

	hash := common.HexToHash(txHash)
	return GenerateProof(context.Background(), hash, rpcClient)
}

// eth_getTransactionByHash
// "eth_getBlockByNumber"
// "eth_getTransactionReceipt"
//

type Substrate struct {
	Tx    json.RawMessage   `json:"tx"`
	Block json.RawMessage   `json:"block"`
	Rcps  []json.RawMessage `json:"rcps"`
}

func GenerateProof(ctx context.Context, txHash common.Hash, rpcClient *Client) ([]byte, error) {
	calldat := rpcClient.NewCallData("eth_getTransactionByHash")
	calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
	calldat.Command.Params = []interface{}{txHash.String()}
	myTx := new(RpcTransaction)
	err := rpcClient.actualRpcCall(calldat, myTx)
	if err != nil {

		return nil, err
	}
	//bl, tx, err := utils.BlockNumberByTransactionHash(ctx, client, txHash)
	//if err != nil {
	//	fmt.Printf("Error: couldn't find block by tx hash: %s\n", err)
	//	return []byte{}, err
	//}

	// Convert returned blocknumber
	var blockNumber big.Int
	blockNumber.SetString((*myTx.BlockNumber)[2:], 16)
	calldat = rpcClient.NewCallData("eth_getBlockByNumber")
	calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
	calldat.Command.Params = []interface{}{myTx.BlockNumber, true}
	myblock := new(json.RawMessage)
	err = rpcClient.actualRpcCall(calldat, myblock)
	if err != nil {

		return nil, err
	}
	block, err := parseBlock(*myblock)
	if err != nil {

		return nil, err
	}
	var idx byte
	txs := block.Transactions()
	txTrie := utils.TxTrie(txs)
	var blockReceipts []*types.Receipt
	for _, tx := range block.Transactions() {
		//receipt, err := ec.TransactionReceipt(context.Background(), tx.Hash())
		receipt := new(types.Receipt)
		calldat = rpcClient.NewCallData("eth_getTransactionReceipt")
		calldat.Context.TargetRPCEndpoint = rpcClient.DefaultRPCEndpoint
		calldat.Command.Params = []interface{}{tx.Hash().String()}
		err = rpcClient.actualRpcCall(calldat, receipt)
		if err != nil {
			return nil, err
		}
		blockReceipts = append(blockReceipts, receipt)
	}

	receiptTrie := utils.ReceiptTrie(blockReceipts)

	// Calculate transaction index)
	for i := 0; i < len(txs); i++ {
		if txHash == txs[i].Hash() {
			idx = byte(i)
		}
	}

	txPath := []byte{idx}
	txRLP, _ := rlp.EncodeToBytes(myTx.Tx)
	txProof := utils.Proof(txTrie, txPath[:])
	receiptRLP, _ := rlp.EncodeToBytes(blockReceipts[txPath[0]])
	receiptProof := utils.Proof(receiptTrie, txPath[:])

	var decodedTx, decodedTxProof, decodedReceipt, decodedReceiptProof []interface{}

	err = rlp.DecodeBytes(txRLP, &decodedTx)
	if err != nil {
		return []byte{}, err
	}

	err = rlp.DecodeBytes(txProof, &decodedTxProof)
	if err != nil {
		return []byte{}, err
	}

	err = rlp.DecodeBytes(receiptRLP, &decodedReceipt)
	if err != nil {
		return []byte{}, err
	}

	err = rlp.DecodeBytes(receiptProof, &decodedReceiptProof)
	if err != nil {
		return []byte{}, err
	}

	proof := make([]interface{}, 0)
	proof = append(proof, txPath, decodedTx, decodedTxProof, decodedReceipt, decodedReceiptProof)

	return rlp.EncodeToBytes(proof)
}

type RpcTransaction struct {
	Tx *types.Transaction
	TxExtraInfo
}

type TxExtraInfo struct {
	BlockNumber *string         `json:"blockNumber,omitempty"`
	BlockHash   *common.Hash    `json:"blockHash,omitempty"`
	From        *common.Address `json:"from,omitempty"`
}

func (tx *RpcTransaction) UnmarshalJSON(msg []byte) error {
	if err := json.Unmarshal(msg, &tx.Tx); err != nil {
		return err
	}
	return json.Unmarshal(msg, &tx.TxExtraInfo)
}

type RpcBlock struct {
	Hash         common.Hash      `json:"hash"`
	Transactions []RpcTransaction `json:"transactions"`
	UncleHashes  []common.Hash    `json:"uncles"`
}

func parseSubstrate(substrate *Substrate) ([]byte, error) {
	myTx := new(RpcTransaction)
	err := json.Unmarshal(substrate.Tx, myTx)
	if err != nil {

		return nil, err
	}

	block, err := parseBlock(substrate.Block)
	if err != nil {
		return nil, err
	}
	txs := block.Transactions()
	txTrie := utils.TxTrie(txs)
	var idx byte
	for i := 0; i < len(txs); i++ {
		if myTx.Tx.Hash() == txs[i].Hash() {
			idx = byte(i)
		}
	}
	var blockReceipts = make([]*types.Receipt, len(txs))
	for i, rraw := range substrate.Rcps {
		rect := new(types.Receipt)
		err = json.Unmarshal(rraw, rect)
		if err != nil {
			return nil, err
		}
		blockReceipts[i] = rect
	}

	receiptTrie := utils.ReceiptTrie(blockReceipts)
	txPath := []byte{idx}
	txRLP, _ := rlp.EncodeToBytes(myTx.Tx)
	txProof := utils.Proof(txTrie, txPath[:])
	receiptRLP, _ := rlp.EncodeToBytes(blockReceipts[txPath[0]])
	receiptProof := utils.Proof(receiptTrie, txPath[:])

	var decodedTx, decodedTxProof, decodedReceipt, decodedReceiptProof []interface{}

	err = rlp.DecodeBytes(txRLP, &decodedTx)
	if err != nil {
		return []byte{}, err
	}

	err = rlp.DecodeBytes(txProof, &decodedTxProof)
	if err != nil {
		return []byte{}, err
	}

	err = rlp.DecodeBytes(receiptRLP, &decodedReceipt)
	if err != nil {
		return []byte{}, err
	}

	err = rlp.DecodeBytes(receiptProof, &decodedReceiptProof)
	if err != nil {
		return []byte{}, err
	}

	proof := make([]interface{}, 0)
	proof = append(proof, txPath, decodedTx, decodedTxProof, decodedReceipt, decodedReceiptProof)

	return rlp.EncodeToBytes(proof)

}

func parseBlock(raw json.RawMessage) (*types.Block, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("No data")
	}
	// Decode header and transactions.
	var head *types.Header
	var body RpcBlock
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	// Quick-verify transaction and uncle lists. This mostly helps with debugging the server.
	if head.UncleHash == types.EmptyUncleHash && len(body.UncleHashes) > 0 {
		return nil, fmt.Errorf("server returned non-empty uncle list but block header indicates no uncles")
	}
	if head.UncleHash != types.EmptyUncleHash && len(body.UncleHashes) == 0 {
		return nil, fmt.Errorf("server returned empty uncle list but block header indicates uncles")
	}
	if head.TxHash == types.EmptyRootHash && len(body.Transactions) > 0 {
		return nil, fmt.Errorf("server returned non-empty transaction list but block header indicates no transactions")
	}
	if head.TxHash != types.EmptyRootHash && len(body.Transactions) == 0 {
		return nil, fmt.Errorf("server returned empty transaction list but block header indicates transactions")
	}
	// Load uncles because they are not included in the block response.
	/* Skipping Uncles ************************************
	var uncles []*types.Header
	if len(body.UncleHashes) > 0 {
		uncles = make([]*types.Header, len(body.UncleHashes))
		reqs := make([]rpc.BatchElem, len(body.UncleHashes))
		for i := range reqs {
			reqs[i] = rpc.BatchElem{
				Method: "eth_getUncleByBlockHashAndIndex",
				Args:   []interface{}{body.Hash, hexutil.EncodeUint64(uint64(i))},
				Result: &uncles[i],
			}
		}
		if err := ec.c.BatchCallContext(ctx, reqs); err != nil {
			return nil, err
		}
		for i := range reqs {
			if reqs[i].Error != nil {
				return nil, reqs[i].Error
			}
			if uncles[i] == nil {
				return nil, fmt.Errorf("got null header for uncle %d of block %x", i, body.Hash[:])
			}
		}
	}
	*/
	uncles := []*types.Header{}

	// Fill the sender cache of transactions in the block.
	txs := make([]*types.Transaction, len(body.Transactions))
	for i, tx := range body.Transactions {
		if tx.From != nil {
			setSenderFromServer(tx.Tx, *tx.From, body.Hash)

		}
		txs[i] = tx.Tx
	}
	return types.NewBlockWithHeader(head).WithBody(txs, uncles), nil
}

type senderFromServer struct {
	addr      common.Address
	blockhash common.Hash
}

func (s *senderFromServer) Equal(other types.Signer) bool {
	os, ok := other.(*senderFromServer)
	return ok && os.blockhash == s.blockhash
}

func (s *senderFromServer) Sender(tx *types.Transaction) (common.Address, error) {
	if s.blockhash == (common.Hash{}) {
		return common.Address{}, fmt.Errorf("Some error")
	}
	return s.addr, nil
}

func (s *senderFromServer) Hash(tx *types.Transaction) common.Hash {
	panic("can't sign with senderFromServer")
}
func (s *senderFromServer) SignatureValues(tx *types.Transaction, sig []byte) (R, S, V *big.Int, err error) {
	panic("can't sign with senderFromServer")
}

func setSenderFromServer(tx *types.Transaction, addr common.Address, block common.Hash) {
	// Use types.Sender for side-effect to store our signer into the cache.
	types.Sender(&senderFromServer{addr, block}, tx)
}

func SubmitSubstrate(data *templates.RenderData, r *http.Request) {
	sub := new(Substrate)
	raw := r.FormValue("Substrate")
	if len(raw) == 0 {
		fmt.Println("No substrate")
		return
	}
	err := json.Unmarshal([]byte(raw), sub)
	if err != nil {
		fmt.Println("Bad substrate:", err)
		return
	}
	proof, err := parseSubstrate(sub)
	if err != nil {
		fmt.Println("Bad substrate(2):", err)
		return
	}
	data.TemplateName = "boottextarea"
	data.BodyData = hex.EncodeToString(proof)
}
