//go:build ion
// +build ion

package proofs

import (
	"context"
	"github.com/clearmatics/ion-cli/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"strings"
)

func GetIonProof(url string, txHash string) ([]byte, error) {
	if !strings.HasPrefix(url, "http") {
		url = "http://" + url
	}
	client, err := rpc.Dial(url)
	if err != nil {
		return nil, err
	}
	hash := common.HexToHash(txHash)
	return utils.GenerateProof(context.Background(), client, hash)
}
