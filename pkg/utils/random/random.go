package random

import (
	"math/rand"
	"time"
)

var initialised = false
var validchars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// As per https://github.com/interconnectedcloud/qdr-operator/blob/1.0.0-beta6/pkg/utils/random/random.go
func GenerateRandomString(n int) string {
	if !initialised {
		rand.Seed(time.Now().UnixNano())
		initialised = true
	}
	b := make([]rune, n)
	for i := range b {
		b[i] = validchars[rand.Intn(len(validchars))]
	}
	return string(b)
}
