package main

import (
	"encoding/hex"
	"fmt"
)

func trimPubKey(pubkey []byte) string {
	nSplit := 8
	if len(pubkey) > nSplit {
		return fmt.Sprintf("%s..%s", hex.EncodeToString(pubkey)[:nSplit/2], hex.EncodeToString(pubkey)[len(hex.EncodeToString(pubkey))-nSplit/2:])
	}
	return hex.EncodeToString(pubkey)
}
