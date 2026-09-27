package ingest

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxFileBytes = 2 << 20

var ErrUnsafe = errors.New("source is not safe for ingestion")

// FixtureText is compiled into the server. Callers cannot supply fixture content.
const FixtureText = "Синтетический пример: доставка заказа подтверждается только после проверки сотрудником."
const FixtureOrigin = "fixture:staffy-synthetic-v1"
const FixtureTextV2 = "Синтетический пример: доставка заказа подтверждается после проверки сотрудником и обновления статуса."

func SyntheticText(fixtureID string) (string, error) {
	switch fixtureID {
	case "sample-v1":
		return FixtureText, nil
	case "sample-v2":
		return FixtureTextV2, nil
	default:
		return "", ErrUnsafe
	}
}

func ValidateFile(name string, data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxFileBytes || len(name) == 0 || len(name) > 255 || filepath.Base(name) != name || strings.ContainsAny(name, "\\/\x00") {
		return "", ErrUnsafe
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt":
		if len(data) > 256<<10 || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 || bytes.HasPrefix(bytes.TrimSpace(bytes.ToLower(data)), []byte("<")) {
			return "", ErrUnsafe
		}
		return "text/plain", nil
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) || !bytes.Contains(data[len(data)-min(len(data), 1024):], []byte("%%EOF")) {
			return "", ErrUnsafe
		}
		return "application/pdf", nil
	default:
		return "", ErrUnsafe
	}
}

// ValidateURLSyntax performs no network access. Until AG-4 passes, real URLs are
// only recorded in quarantine; even a DNS lookup would be premature egress.
func ValidateURLSyntax(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Port() != "" && u.Port() != "443") || len(raw) > 2048 {
		return nil, ErrUnsafe
	}
	if strings.HasSuffix(strings.ToLower(u.Hostname()), ".local") || !strings.Contains(u.Hostname(), ".") {
		return nil, ErrUnsafe
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		if err := ValidateAddresses([]net.IP{ip}); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// ValidateURL is for a future approved fetcher: check every redirect and pin
// the returned address in the dialer, never resolving the host a second time.
func ValidateURL(ctx context.Context, raw string, resolver *net.Resolver) (*url.URL, []net.IP, error) {
	u, err := ValidateURLSyntax(raw)
	if err != nil {
		return nil, nil, err
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addresses, err := resolver.LookupIP(ctx, "ip", u.Hostname())
	if err != nil || len(addresses) == 0 {
		return nil, nil, ErrUnsafe
	}
	if err := ValidateAddresses(addresses); err != nil {
		return nil, nil, err
	}
	return u, addresses, nil
}

func ValidateAddresses(addresses []net.IP) error {
	if len(addresses) == 0 {
		return ErrUnsafe
	}
	for _, ip := range addresses {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
			return ErrUnsafe
		}
	}
	return nil
}
