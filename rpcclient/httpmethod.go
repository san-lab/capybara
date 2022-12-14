package rpcclient

import (
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/san-lab/capybara/templates"
)

func (rpcCleint *Client) IonProof(data *templates.RenderData, rq *http.Request, w http.ResponseWriter) {
	shash := rq.FormValue("tx_hash")
	if len(shash) == 0 {
		fmt.Fprintln(w, "No hash given")
		return
	}

	fmt.Fprintln(w, shash)
	proof, err := GetIonProof(rpcCleint, shash)
	fmt.Fprintln(w, err)
	fmt.Fprintln(w, hex.EncodeToString(proof))

}

func GetProof(rpcClient *Client, th *TxH) error {
	if th.Tx != nil {
		var err error

		b, err := GetIonProof(rpcClient, th.Tx.Hash)
		if err != nil {

			return err
		}
		th.Pr = hex.EncodeToString(b)

	}
	return nil
}
