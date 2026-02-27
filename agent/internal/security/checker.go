package security

import (
	"context"
	"crypto/tls"
	"math"
	"net"
	"net/url"
	"time"

	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"

	"github.com/obsidianstack/obsidianstack/agent/internal/config"
)

// Check dials the TLS endpoint for the given source and returns a CertStatus
// describing the leaf certificate.
//
// Returns nil for non-HTTPS endpoints — there is no TLS certificate to inspect.
// Uses a 10-second dial timeout so a slow/unreachable host does not block the
// scrape loop indefinitely.
func Check(ctx context.Context, src config.Source) *pb.CertStatus {
	u, err := url.Parse(src.Endpoint)
	if err != nil || u.Scheme != "https" {
		return nil // nothing to inspect for plain-HTTP or unparseable endpoints
	}

	cs := &pb.CertStatus{
		Endpoint: src.Endpoint,
		AuthType: src.Auth.Mode,
	}
	if cs.AuthType == "" {
		cs.AuthType = "none"
	}

	host := u.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		// No explicit port in the URL — append the HTTPS default.
		host = net.JoinHostPort(host, "443")
	}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{},
		Config: &tls.Config{
			InsecureSkipVerify: src.TLS.InsecureSkipVerify, //nolint:gosec
		},
	}

	netConn, err := dialer.DialContext(dialCtx, "tcp", host)
	if err != nil {
		cs.Status = "unreachable"
		return cs
	}
	conn := netConn.(*tls.Conn)
	defer conn.Close()

	peerCerts := conn.ConnectionState().PeerCertificates
	if len(peerCerts) == 0 {
		cs.Status = "unreachable"
		return cs
	}

	leaf := peerCerts[0]
	now := time.Now()
	daysLeft := leaf.NotAfter.Sub(now).Hours() / 24

	cs.NotAfter = leaf.NotAfter.UTC().Format(time.RFC3339)
	cs.Issuer = leaf.Issuer.CommonName
	cs.DaysLeft = int32(math.Floor(daysLeft))

	switch {
	case daysLeft <= 0:
		cs.Status = "expired"
	case daysLeft <= 30:
		cs.Status = "expiring"
	default:
		cs.Status = "valid"
	}

	return cs
}
