package id

import (
	"crypto/rand"
	"encoding/hex"
)

const byteLength = 16

func GenerateID() string {
	bytes := make([]byte, byteLength)
	if _, err := rand.Read(bytes); err != nil {
		panic("generating random ID: " + err.Error())
	}

	return hex.EncodeToString(bytes)
}
