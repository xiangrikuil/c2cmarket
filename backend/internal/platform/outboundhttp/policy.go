package outboundhttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
)

var (
	ErrInvalidTarget      = errors.New("outbound target is invalid")
	ErrHostNotAllowed     = errors.New("outbound target host is not allowed")
	ErrResolutionFailed   = errors.New("outbound target resolution failed")
	ErrUnsafeAddress      = errors.New("outbound target resolved to an unsafe address")
	ErrDialFailed         = errors.New("outbound target connection failed")
	ErrRedirectNotAllowed = errors.New("outbound redirect is not allowed")
	ErrResponseTooLarge   = errors.New("outbound response exceeds the configured limit")
)

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/96"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
	netip.MustParsePrefix("5f00::/16"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("fec0::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

type Policy struct {
	resolver        Resolver
	dialer          Dialer
	allowedHosts    map[string]struct{}
	maxDialAttempts int
	stats           policyStats
}

type PolicyStats struct {
	InvalidTargetTotal    uint64
	HostNotAllowedTotal   uint64
	ResolutionFailedTotal uint64
	UnsafeAddressTotal    uint64
	RedirectRejectedTotal uint64
}

type policyStats struct {
	invalidTarget    atomic.Uint64
	hostNotAllowed   atomic.Uint64
	resolutionFailed atomic.Uint64
	unsafeAddress    atomic.Uint64
	redirectRejected atomic.Uint64
}

type PolicyOption func(*Policy)

func WithResolver(resolver Resolver) PolicyOption {
	return func(policy *Policy) {
		if resolver != nil {
			policy.resolver = resolver
		}
	}
}

func WithDialer(dialer Dialer) PolicyOption {
	return func(policy *Policy) {
		if dialer != nil {
			policy.dialer = dialer
		}
	}
}

func NewPolicy(allowedHosts []string, options ...PolicyOption) (*Policy, error) {
	policy := &Policy{
		resolver:        net.DefaultResolver,
		dialer:          &net.Dialer{Timeout: defaultDialTimeout},
		allowedHosts:    map[string]struct{}{},
		maxDialAttempts: 3,
	}
	for _, option := range options {
		option(policy)
	}
	for _, value := range allowedHosts {
		host, err := normalizeHost(strings.TrimSpace(value))
		if err != nil || strings.Contains(value, "*") {
			return nil, fmt.Errorf("%w: allowlist entry", ErrInvalidTarget)
		}
		policy.allowedHosts[host] = struct{}{}
	}
	return policy, nil
}

func (p *Policy) ValidateURL(ctx context.Context, raw string) (string, error) {
	normalized, host, err := p.normalizeURL(raw)
	if err != nil {
		p.recordRejection(err)
		return "", err
	}
	if _, err := p.resolve(ctx, host); err != nil {
		p.recordRejection(err)
		return "", err
	}
	return normalized, nil
}

func (p *Policy) normalizeURL(raw string) (string, string, error) {
	if p == nil {
		return "", "", ErrInvalidTarget
	}
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil || value == "" || parsed.Opaque != "" || !parsed.IsAbs() {
		return "", "", ErrInvalidTarget
	}
	if !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return "", "", ErrInvalidTarget
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", "", ErrInvalidTarget
	}

	rawHost := parsed.Hostname()
	host, err := normalizeHost(rawHost)
	if err != nil {
		return "", "", ErrInvalidTarget
	}
	port, err := normalizePort(parsed.Host, parsed.Port())
	if err != nil {
		return "", "", ErrInvalidTarget
	}
	if err := p.validateAllowedHost(host); err != nil {
		return "", "", err
	}

	parsed.Scheme = "https"
	parsed.User = nil
	parsed.Host = canonicalAuthority(host, port)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed.String(), host, nil
}

func (p *Policy) validateRequestURL(target *url.URL) error {
	if target == nil {
		return ErrInvalidTarget
	}
	_, _, err := p.normalizeURL(target.String())
	return err
}

func (p *Policy) validateAllowedHost(host string) error {
	if p == nil {
		return ErrInvalidTarget
	}
	if len(p.allowedHosts) == 0 {
		return nil
	}
	if _, ok := p.allowedHosts[host]; !ok {
		return ErrHostNotAllowed
	}
	return nil
}

func (p *Policy) resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	if p == nil || p.resolver == nil {
		return nil, ErrResolutionFailed
	}
	if literal, err := netip.ParseAddr(host); err == nil {
		if !isPublicAddress(literal) {
			return nil, ErrUnsafeAddress
		}
		return []netip.Addr{literal}, nil
	}

	addresses, err := p.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrResolutionFailed
	}
	result := make([]netip.Addr, 0, len(addresses))
	seen := map[netip.Addr]struct{}{}
	for _, address := range addresses {
		if !isPublicAddress(address) {
			return nil, ErrUnsafeAddress
		}
		address = address.Unmap()
		if _, ok := seen[address]; ok {
			continue
		}
		seen[address] = struct{}{}
		result = append(result, address)
	}
	if len(result) == 0 {
		return nil, ErrResolutionFailed
	}
	return result, nil
}

func (p *Policy) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, ErrDialFailed
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrDialFailed
	}
	host, err = normalizeHost(host)
	if err != nil {
		return nil, ErrDialFailed
	}
	if _, err := normalizePort(net.JoinHostPort(host, port), port); err != nil {
		return nil, ErrDialFailed
	}
	if err := p.validateAllowedHost(host); err != nil {
		p.recordRejection(err)
		return nil, err
	}
	addresses, err := p.resolve(ctx, host)
	if err != nil {
		p.recordRejection(err)
		return nil, err
	}
	if p.dialer == nil {
		return nil, ErrDialFailed
	}

	attempts := len(addresses)
	if attempts > p.maxDialAttempts {
		attempts = p.maxDialAttempts
	}
	for _, resolved := range addresses[:attempts] {
		conn, dialErr := p.dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, ErrDialFailed
}

func (p *Policy) Stats() PolicyStats {
	if p == nil {
		return PolicyStats{}
	}
	return PolicyStats{
		InvalidTargetTotal:    p.stats.invalidTarget.Load(),
		HostNotAllowedTotal:   p.stats.hostNotAllowed.Load(),
		ResolutionFailedTotal: p.stats.resolutionFailed.Load(),
		UnsafeAddressTotal:    p.stats.unsafeAddress.Load(),
		RedirectRejectedTotal: p.stats.redirectRejected.Load(),
	}
}

func (p *Policy) RecordRedirectRejection() {
	if p != nil {
		p.stats.redirectRejected.Add(1)
	}
}

func (p *Policy) recordRejection(err error) {
	if p == nil || err == nil {
		return
	}
	switch {
	case errors.Is(err, ErrHostNotAllowed):
		p.stats.hostNotAllowed.Add(1)
	case errors.Is(err, ErrUnsafeAddress):
		p.stats.unsafeAddress.Add(1)
	case errors.Is(err, ErrResolutionFailed):
		p.stats.resolutionFailed.Add(1)
	case errors.Is(err, ErrInvalidTarget):
		p.stats.invalidTarget.Add(1)
	}
}

func normalizeHost(value string) (string, error) {
	host := strings.TrimSpace(value)
	if strings.HasPrefix(host, "[") || strings.HasSuffix(host, "]") {
		if !strings.HasPrefix(host, "[") || !strings.HasSuffix(host, "]") {
			return "", ErrInvalidTarget
		}
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	if host == "" || strings.ContainsAny(host, "[]/%@\\") {
		return "", ErrInvalidTarget
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if address.Zone() != "" || address.Is4In6() {
			return "", ErrInvalidTarget
		}
		return address.String(), nil
	}
	if strings.Contains(host, ":") {
		return "", ErrInvalidTarget
	}

	host = strings.ToLower(host)
	if strings.HasSuffix(host, ".") {
		host = strings.TrimSuffix(host, ".")
	}
	if host == "" || strings.HasSuffix(host, ".") || len(host) > 253 {
		return "", ErrInvalidTarget
	}
	if looksLikeAlternativeIPv4(host) {
		return "", ErrInvalidTarget
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrInvalidTarget
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return "", ErrInvalidTarget
			}
		}
	}
	return host, nil
}

func looksLikeAlternativeIPv4(host string) bool {
	labels := strings.Split(host, ".")
	if len(labels) > 4 {
		return false
	}
	for _, label := range labels {
		if label == "" {
			return false
		}
		if strings.HasPrefix(label, "0x") {
			if len(label) == 2 {
				return false
			}
			for _, char := range label[2:] {
				if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
					return false
				}
			}
			continue
		}
		for _, char := range label {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func normalizePort(authority, port string) (string, error) {
	if strings.HasSuffix(authority, ":") {
		return "", ErrInvalidTarget
	}
	if port == "" {
		if !strings.HasPrefix(authority, "[") && strings.Contains(authority, ":") {
			return "", ErrInvalidTarget
		}
		return "", nil
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 || strconv.Itoa(value) != port {
		return "", ErrInvalidTarget
	}
	return port, nil
}

func canonicalAuthority(host, port string) string {
	if port != "" {
		return net.JoinHostPort(host, port)
	}
	if address, err := netip.ParseAddr(host); err == nil && address.Is6() {
		return "[" + host + "]"
	}
	return host
}

func isPublicAddress(address netip.Addr) bool {
	if !address.IsValid() || address.Zone() != "" || address.Is4In6() {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || address.IsUnspecified() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}
