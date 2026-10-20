package store

import (
	"crypto/rand"
	"encoding/hex"
)

// newID returns a random 16-byte identifier as hex, with the shape the
// Electron build's UUIDs had: unique, ordered by nothing, safe in a file
// name.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("store: reading random bytes: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
