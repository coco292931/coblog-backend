package userService

import (
	"crypto/sha256"
	"encoding/base64"
	"time"
)

func GenToken(email string) string {
		//rssToken
	hash := sha256.Sum256([]byte(email + time.Now().String()))
	token := base64.RawURLEncoding.EncodeToString(hash[:12])

	return token
}