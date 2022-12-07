package proofs

import (
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/san-lab/capybara/templates"
)

func IonProof(data *templates.RenderData, rq *http.Request, w http.ResponseWriter, url string) {
	shash := rq.FormValue("tx_hash")
	if len(shash) == 0 {
		fmt.Fprintln(w, "No hash given")
		return
	}

	fmt.Fprintln(w, shash)
	proof, err := GetIonProof(url, shash)
	fmt.Fprintln(w, err)
	fmt.Fprintln(w, hex.EncodeToString(proof))

}
