package ingest

import (
	"context"
	"errors"
)

var ErrScannerUnavailable = errors.New("scanner unavailable")

// Scanner is the admission boundary for future real sources. No implementation
// may mark a source safe without verified malware and safe-extraction evidence.
type Scanner interface {
	ScanFile(context.Context, string, []byte) error
	ScanPage(context.Context, string, []byte) error
}

type UnavailableScanner struct{}

func (UnavailableScanner) ScanFile(context.Context, string, []byte) error {
	return ErrScannerUnavailable
}
func (UnavailableScanner) ScanPage(context.Context, string, []byte) error {
	return ErrScannerUnavailable
}
