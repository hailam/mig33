package accounts

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/crypto/pbkdf2"
)

// DevelopmentOTP deliberately remains static for the local reconstruction.
const (
	DevelopmentOTP     = "123456"
	passwordIterations = 600000
)

func PasswordIdentity(name, password string) (Identity, error) {
	a := Identity{Username: strings.ToLower(name)}
	if !utf8.ValidString(password) || strings.ContainsRune(password, 0) || len(utf16.Encode([]rune(password))) < 4 || len(utf16.Encode([]rune(password))) > 320 {
		return a, fmt.Errorf("Use a password of 4 to 320 characters")
	}
	password = strings.ToLower(password) // Original Java client lowercases before 202.
	a.PasswordSalt = make([]byte, 16)
	if _, err := rand.Read(a.PasswordSalt); err != nil {
		return a, err
	}
	a.PasswordIterations = passwordIterations
	a.PasswordKey = pbkdf2.Key([]byte(password), a.PasswordSalt, a.PasswordIterations, 32, sha256.New)
	a.LegacyHash = JavaHash(password)
	a.LegacyUnits = len(utf16.Encode([]rune(password)))
	return a, nil
}

func VerifyPassword(a Identity, password string) bool {
	if len(password) > 1280 || !utf8.ValidString(password) {
		return false
	}
	key := pbkdf2.Key([]byte(strings.ToLower(password)), a.PasswordSalt, a.PasswordIterations, 32, sha256.New)
	return subtle.ConstantTimeCompare(key, a.PasswordKey) == 1
}

func LegacyProof(a Identity, challenge string) int32 {
	h := uint32(JavaHash(challenge))
	for i := 0; i < a.LegacyUnits; i++ {
		h *= 31
	}
	return int32(h + uint32(a.LegacyHash))
}

func WithPassword(a, credentials Identity) Identity {
	a.PasswordSalt = credentials.PasswordSalt
	a.PasswordKey = credentials.PasswordKey
	a.PasswordIterations = credentials.PasswordIterations
	a.LegacyHash = credentials.LegacyHash
	a.LegacyUnits = credentials.LegacyUnits
	return a
}

func NormalizeName(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) < 3 || len(name) > 32 {
		return "", fmt.Errorf("Username must contain 3 to 32 letters, numbers, dots, hyphens or underscores")
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '-') {
			return "", fmt.Errorf("Username contains an unsupported character")
		}
	}
	return name, nil
}

func NormalizeMobile(mobile string) (string, error) {
	mobile = strings.TrimSpace(mobile)
	mobile = strings.TrimPrefix(mobile, "+")
	if len(mobile) < 6 || len(mobile) > 20 {
		return "", fmt.Errorf("Enter a mobile number with 6 to 20 digits, including country code")
	}
	for _, r := range mobile {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("Enter digits and an optional leading +")
		}
	}
	return "+" + mobile, nil
}

// JavaHash reproduces Java String.hashCode over UTF-16 code units.
func JavaHash(value string) int32 {
	var hash uint32
	for _, unit := range utf16.Encode([]rune(value)) {
		hash = hash*31 + uint32(unit)
	}
	return int32(hash)
}
