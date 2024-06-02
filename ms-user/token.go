package main

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateJwt(userId string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userId,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
	})

	return token.SignedString([]byte("mysecret"))
}

func generateOpaque() (string, error) {
	token := make([]byte, 100)

	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}

	opaque := base64.URLEncoding.EncodeToString(token)

	return opaque, nil
}
