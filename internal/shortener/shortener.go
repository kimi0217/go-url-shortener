// Package shortener generates random short codes.
package shortener

import (
	"crypto/rand"
	"math/big"
)

// Length is the number of characters in a generated code.
// 62^6 ≈ 5.7e10 possible codes keeps collisions rare at this project's scale.
const Length = 6

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// alphabetSize is computed once rather than on every character.
var alphabetSize = big.NewInt(int64(len(alphabet)))

// Generate returns a random code of Length alphanumeric characters.
//
// crypto/rand is used instead of math/rand so codes are unpredictable:
// someone who sees a few codes cannot guess others and enumerate the
// URLs people have shortened.
//
// Generate does not check for collisions; the caller handles that by
// retrying when the store reports the code is already taken.
func Generate() (string, error) {
	code := make([]byte, Length)
	for i := range code {
		// rand.Int returns a uniform value in [0, 62). The obvious shortcut,
		// randomByte % 62, would be biased: 256 is not a multiple of 62, so
		// the first 8 characters would come up slightly more often.
		n, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", err
		}
		code[i] = alphabet[n.Int64()]
	}
	return string(code), nil
}
