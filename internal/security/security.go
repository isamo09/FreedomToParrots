// Package security provides random tokens and constant-time comparisons
// used for the panel password and login cookies.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
)

// passwordAlphabet avoids visually ambiguous characters (0/O, 1/l/I) so a
// freshly generated password is easy to read off a terminal and retype.
const passwordAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"

// NewPassword returns a random human-typeable password, grouped as XXXX-XXXX-XXXX.
func NewPassword() string {
	const groups, groupLen = 3, 4

	b := make([]byte, groups*groupLen)
	randomBytes(b)

	out := make([]byte, 0, groups*groupLen+groups-1)

	for i, c := range b {
		if i > 0 && i%groupLen == 0 {
			out = append(out, '-')
		}

		out = append(out, passwordAlphabet[int(c)%len(passwordAlphabet)])
	}

	return string(out)
}

// NewToken returns a random hex token suitable for session ids and session keys.
func NewToken(nBytes int) string {
	b := make([]byte, nBytes)
	randomBytes(b)

	return hex.EncodeToString(b)
}

func randomBytes(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic("security: system randomness unavailable: " + err.Error())
	}
}

// Equal does a constant-time comparison of two secrets.
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
