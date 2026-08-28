package kernel

import (
	"crypto/rand"
	"fmt"
)

// NewID mints a random UUIDv4 for a new entity. ID generation is stateful and
// impure, so it belongs to the framework; feature wiring passes the result into
// a pure planning function, keeping that function free of hidden inputs.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is catastrophic and not something a request can
		// meaningfully recover from.
		panic("kernel.NewID: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
