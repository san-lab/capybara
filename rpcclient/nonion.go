//go:build !ion

package rpcclient

import (
	"fmt"
)

func xGetIonProof(url string, txHash string) ([]byte, error) {

	return nil, fmt.Errorf("Not an Ion build")

}
