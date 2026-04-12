package relay

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func randomID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buf)), nil
}
