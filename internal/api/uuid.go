package api

import (
	"crypto/rand"
	"fmt"
)

// newID generates a UUID v4 string for use as a unique identifier.
// Uses crypto/rand so no external dependencies are needed.
func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
