package services

import (
	"crypto/rand"
	"math/big"
)

const (
	alphabet string = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	codeLen  int    = 6 // six-character enrollment codes
)

/*
newEnrollmentCode returns a cryptographically random six-character code. The
code is the class-join authorization secret, so it must not be derivable from
the offering's sequential id or from another class's code.
*/
func newEnrollmentCode() (string, error) {
	chars := make([]byte, codeLen)
	limit := big.NewInt(int64(len(alphabet)))

	for i := range chars {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		chars[i] = alphabet[n.Int64()]
	}

	return string(chars), nil
}
