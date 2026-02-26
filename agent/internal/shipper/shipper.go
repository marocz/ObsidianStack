package shipper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/obsidianstack/obsidianstack/agent/internal/compute"
	"github.com/obsidianstack/obsidianstack/agent/internal/config"
	pb "github.com/obsidianstack/obsidianstack/gen/obsidian/v1"
)

const (
	backoffInitial    = 1 * time.Second
	backoffMax        = 60 * time.Second
	backoffMultiplier = 2.0
	sendTimeout       = 10 * time.Second
)

// Shipper buffers compute.Results and ships them to obsidianstack-server via gRPC.
// Ship() is non-blocking; when the buffer is full the oldest snapshot is evicted.
// Run() must be called in a goroutine to drain the buffer and handle reconnection.
type Shipper struct {
	cfg    config.AgentConfig
	buf    chan *pb.PipelineSnapshot
	dialFn dialFunc // injectable for tests
}

// dialFunc is the function signature used to open a gRPC connection.
// Abstracted so tests can inject an in-memory bufconn dialer.
type dialFunc func(ctx context.Context, endpoint string, cfg config.AgentConfig) (*grpc.ClientConn, error)

// New creates a Shipper using the given agent config.
func New(cfg config.AgentConfig) *Shipper {
	return &Shipper{
		cfg:    cfg,
		buf:    make(chan *pb.PipelineSnapshot, cfg.BufferSize),
		dialFn: defaultDial,
	}
}

// Ship converts a compute.Result to a proto snapshot and enqueues it.
// If the buffer is full the oldest entry is evicted to make room.
func (s *Shipper) Ship(res *compute.Result) {
	snap := toProto(res)
	select {
	case s.buf <- snap:
	default:
		// Buffer full — drop the oldest snapshot, keep the newest.
		select {
		case <-s.buf:
			slog.Warn("shipper: buffer full, evicted oldest snapshot",
				"source", res.SourceID, "buffer_cap", cap(s.buf))
		default:
		}
		s.buf <- snap
	}
}

// Run drains the buffer, sending snapshots to the server.
// It reconnects with exponential backoff when the connection is lost.
// Run blocks until ctx is cancelled.
func (s *Shipper) Run(ctx context.Context) {
	bo := newBackoff()

	for {
		if ctx.Err() != nil {
			return
		}

		conn, err := s.dialFn(ctx, s.cfg.ServerEndpoint, s.cfg)
		if err != nil {
			wait := bo.next()
			slog.Error("shipper: dial failed, will retry",
				"endpoint", s.cfg.ServerEndpoint,
				"err", err,
				"retry_in", wait)
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
				continue
			}
		}

		slog.Info("shipper: connected", "endpoint", s.cfg.ServerEndpoint)
		bo.reset()

		err = s.drain(ctx, conn)
		conn.Close()

		if ctx.Err() != nil {
			return
		}

		wait := bo.next()
		slog.Warn("shipper: connection lost, will reconnect",
			"endpoint", s.cfg.ServerEndpoint,
			"err", err,
			"retry_in", wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// drain reads from the buffer and sends snapshots until the connection fails
// or ctx is cancelled.
func (s *Shipper) drain(ctx context.Context, conn *grpc.ClientConn) error {
	client := pb.NewSnapshotServiceClient(conn)

	for {
		select {
		case <-ctx.Done():
			return nil

		case snap := <-s.buf:
			sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)

			// Inject API key header if configured.
			if s.cfg.ServerAuth.Mode == "apikey" && s.cfg.ServerAuth.KeyEnv != "" {
				sendCtx = metadata.AppendToOutgoingContext(
					sendCtx,
					s.cfg.ServerAuth.Header, s.cfg.ServerAuth.Key(),
				)
			}

			resp, err := client.SendSnapshot(sendCtx, snap)
			cancel()

			if err != nil {
				// Put the snapshot back at the front if there's room.
				select {
				case s.buf <- snap:
				default:
					// Buffer full — snapshot lost; this is acceptable since the
					// server will receive the next cycle's data on reconnect.
				}

				// Transient errors (unavailable, deadline exceeded) → reconnect.
				// Permanent errors (unauthenticated, invalid arg) → log and discard.
				if isPermanentError(err) {
					slog.Error("shipper: permanent send error, discarding snapshot",
						"source", snap.SourceId, "err", err)
					continue
				}
				return fmt.Errorf("send: %w", err)
			}

			if !resp.Ok {
				slog.Warn("shipper: server rejected snapshot",
					"source", snap.SourceId, "message", resp.Message)
			} else {
				slog.Debug("shipper: snapshot delivered", "source", snap.SourceId)
			}
		}
	}
}

// isPermanentError returns true for gRPC errors that indicate the snapshot
// itself is invalid and should not be retried.
func isPermanentError(err error) bool {
	code := status.Code(err)
	switch code {
	case codes.InvalidArgument, codes.Unauthenticated, codes.PermissionDenied:
		return true
	}
	return false
}

// defaultDial opens a gRPC connection to endpoint with auth configured from cfg.
func defaultDial(ctx context.Context, endpoint string, cfg config.AgentConfig) (*grpc.ClientConn, error) {
	opts, err := dialOptions(cfg)
	if err != nil {
		return nil, err
	}
	return grpc.DialContext(ctx, endpoint, opts...) //nolint:staticcheck // deprecated in 1.63 but DialContext is used for compat
}

// dialOptions builds grpc.DialOption slice based on the server auth config.
func dialOptions(cfg config.AgentConfig) ([]grpc.DialOption, error) {
	switch cfg.ServerAuth.Mode {
	case "mtls":
		creds, err := buildMTLSCreds(cfg.ServerAuth)
		if err != nil {
			return nil, fmt.Errorf("shipper: build mtls creds: %w", err)
		}
		return []grpc.DialOption{grpc.WithTransportCredentials(creds)}, nil

	case "apikey":
		// API key is injected per-call in drain(); use plain TLS transport.
		// In production you'd also want server-side TLS here.
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, nil

	default: // "none" or empty — insecure for local dev
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, nil
	}
}

// buildMTLSCreds loads client certificate and optional CA from the auth config.
func buildMTLSCreds(auth config.AuthConfig) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(auth.CertFile, auth.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client cert: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	if auth.CAFile != "" {
		caPEM, err := os.ReadFile(auth.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("no valid certs in ca file %q", auth.CAFile)
		}
		tlsCfg.RootCAs = pool
	}

	return credentials.NewTLS(tlsCfg), nil
}

// backoff implements truncated exponential backoff with jitter.
type backoff struct {
	current time.Duration
}

func newBackoff() *backoff {
	return &backoff{current: backoffInitial}
}

// next returns the current backoff duration and advances the internal state.
func (b *backoff) next() time.Duration {
	d := b.current
	// Apply ±25 % jitter.
	jitter := time.Duration(float64(b.current) * 0.25 * (rand.Float64()*2 - 1)) //nolint:gosec // not crypto
	d += jitter
	if d < 0 {
		d = 0
	}

	// Advance for next call.
	b.current = time.Duration(float64(b.current) * backoffMultiplier)
	if b.current > backoffMax {
		b.current = backoffMax
	}
	return d
}

func (b *backoff) reset() {
	b.current = backoffInitial
}
