package apiorder

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestGenerateOrderNoUsesShanghaiDateAndUnambiguousAlphabet(t *testing.T) {
	createdAt := time.Date(2026, 8, 1, 16, 30, 0, 0, time.UTC)
	orderNo, err := generateOrderNo(createdAt, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}))
	if err != nil {
		t.Fatalf("generate order number: %v", err)
	}
	if orderNo != "API-20260802-ABCDEFGHJK" {
		t.Fatalf("unexpected order number: %s", orderNo)
	}
	if strings.ContainsAny(orderNo[len(orderNo)-OrderNumberSuffixLength:], "01ILO") {
		t.Fatalf("order number contains an ambiguous character: %s", orderNo)
	}
}

func TestGenerateOrderNoRejectsBiasedBytesAndSurfacesReaderFailure(t *testing.T) {
	createdAt := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	input := append([]byte{255, 249}, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}...)
	orderNo, err := generateOrderNo(createdAt, bytes.NewReader(input))
	if err != nil {
		t.Fatalf("generate order number after rejection sampling: %v", err)
	}
	if orderNo != "API-20260802-ABCDEFGHJK" {
		t.Fatalf("unexpected order number: %s", orderNo)
	}

	if _, err := generateOrderNo(createdAt, bytes.NewReader([]byte{0, 1})); err == nil {
		t.Fatal("expected a short random source to fail")
	}
}

func TestGenerateOrderNoProductionFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^API-[0-9]{8}-[ABCDEFGHJKMNPQRSTUVWXYZ23456789]{10}$`)
	seen := make(map[string]struct{}, 256)
	createdAt := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	for range 256 {
		orderNo, err := GenerateOrderNo(createdAt)
		if err != nil {
			t.Fatalf("generate production order number: %v", err)
		}
		if !pattern.MatchString(orderNo) {
			t.Fatalf("invalid production order number: %s", orderNo)
		}
		if _, exists := seen[orderNo]; exists {
			t.Fatalf("duplicate order number in test sample: %s", orderNo)
		}
		seen[orderNo] = struct{}{}
	}
}
