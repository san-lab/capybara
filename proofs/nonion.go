//go:build !ion

package proofs

import (
	"fmt"
)

func GetIonProof(url string, txHash string) ([]byte, error) {

	return nil, fmt.Errorf("Not an Ion build")

}
