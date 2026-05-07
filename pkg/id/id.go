package id

import (
	"crypto/rand"
	"encoding/hex"
)

const size = 13

func New() string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}
