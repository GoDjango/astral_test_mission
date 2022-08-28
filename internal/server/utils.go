package server

import (
	"crypto/hmac"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
	"unicode"
	"unicode/utf8"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func GeneratePasswordHash(password string) (string, error) {
	secretKey := []byte("homo sapiens")
	mac := hmac.New(md5.New, secretKey)
	if _, err := mac.Write([]byte(password)); err != nil {
		return "", fmt.Errorf("Failed to create password hash. Error: %s ", err)
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func GenerateSecureToken() (string, error) {
	b := make([]byte, 30)
	if _, err := rand.Read(b); err != nil {
		return "", nil
	}
	return hex.EncodeToString(b), nil
}

func IsPasswordValid(s string) bool {
	var (
		hasMinLen  = false
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)
	if utf8.RuneCountInString(s) >= 8 {
		hasMinLen = true
	}
	for _, char := range s {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}
	return hasMinLen && hasUpper && hasLower && hasNumber && hasSpecial
}
