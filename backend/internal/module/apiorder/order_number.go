package apiorder

import (
	"crypto/rand"
	"fmt"
	"io"
	"time"
)

const (
	OrderNumberAlphabet     = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	OrderNumberSuffixLength = 10
)

var shanghaiTime = time.FixedZone("Asia/Shanghai", 8*60*60)

func GenerateOrderNo(createdAt time.Time) (string, error) {
	return generateOrderNo(createdAt, rand.Reader)
}

func generateOrderNo(createdAt time.Time, random io.Reader) (string, error) {
	if random == nil {
		return "", fmt.Errorf("API order number random source is unavailable")
	}

	suffix := make([]byte, 0, OrderNumberSuffixLength)
	buffer := []byte{0}
	const unbiasedLimit = 248 // 31 * 8; larger bytes would introduce modulo bias.
	for len(suffix) < OrderNumberSuffixLength {
		if _, err := io.ReadFull(random, buffer); err != nil {
			return "", fmt.Errorf("generate API order number: %w", err)
		}
		if int(buffer[0]) >= unbiasedLimit {
			continue
		}
		suffix = append(suffix, OrderNumberAlphabet[int(buffer[0])%len(OrderNumberAlphabet)])
	}

	return "API-" + createdAt.In(shanghaiTime).Format("20060102") + "-" + string(suffix), nil
}
