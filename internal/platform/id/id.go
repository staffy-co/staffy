package id

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var ErrInvalid = errors.New("invalid UUID")

type UUID string

func NewV7(now time.Time) (UUID, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate UUID entropy: %w", err)
	}
	millis := uint64(now.UTC().UnixMilli())
	for i := 5; i >= 0; i-- {
		raw[i] = byte(millis)
		millis >>= 8
	}
	raw[6] = (raw[6] & 0x0f) | 0x70
	raw[8] = (raw[8] & 0x3f) | 0x80
	return format(raw), nil
}

func Parse(value string) (UUID, error) {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return "", ErrInvalid
	}
	compact := value[0:8] + value[9:13] + value[14:18] + value[19:23] + value[24:36]
	raw, err := hex.DecodeString(compact)
	if err != nil || len(raw) != 16 {
		return "", ErrInvalid
	}
	allZero := true
	for _, current := range raw {
		allZero = allZero && current == 0
	}
	if allZero || raw[6]>>4 != 7 || raw[8]&0xc0 != 0x80 {
		return "", ErrInvalid
	}
	return UUID(value), nil
}

func MustParse(value string) UUID {
	parsed, err := Parse(value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func (u UUID) String() string { return string(u) }
func (u UUID) Valid() bool {
	_, err := Parse(string(u))
	return err == nil
}

func format(raw [16]byte) UUID {
	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], raw[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], raw[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], raw[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], raw[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], raw[10:16])
	return UUID(encoded)
}
