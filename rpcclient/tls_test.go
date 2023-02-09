package rpcclient

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestTls(t *testing.T) {
	jcom := `{"jsonrpc":"2.0","method":"txpool_content","params":[],"id":1}`
	host := "https://web3-ws.tmp-demo.polygon-nightfall.technology/"

	req, err := http.NewRequest("POST", host, bytes.NewReader([]byte(jcom)))
	if err != nil {
		t.Error(err)
	}

	rpcClient := http.DefaultClient
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "TLSTestAgent")
	req.Header.Set("Content-type", "application/json")
	resp, err := rpcClient.Do(req)
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("Status code: %v", resp.StatusCode)
	}
	respBytes, err := ioutil.ReadAll(resp.Body)

	if err != nil {

		t.Error(err)
	}

	fmt.Println("Returned:\n" + fmt.Sprintf("%s", resp.Header))
	fmt.Println("Returned:\n" + string(respBytes))
	if err != nil {
		t.Error(err)
	}

}
