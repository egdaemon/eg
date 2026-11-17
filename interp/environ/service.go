// Package environ serves the environment file a workload was provided over the control socket.
package environ

import (
	"context"
	"os"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/golang-jwt/jwt/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// NewToken signs a token for the run, valid for the given duration (the workload ttl), that authenticates calls to the service.
func NewToken(secret jwtx.JWTSecretSource, runid string, d time.Duration) (string, error) {
	return jwtx.Signed(secret(), jwtx.NewJWTClaims(runid, jwtx.ClaimsOptionExpiration(d)))
}

// NewService serves the file at path. calls are authenticated by tokens signed with secret,
// which must not be empty, see cmdopts.EnvironSecret.
func NewService(path string, secret jwtx.JWTSecretSource) *Service {
	return &Service{path: path, secret: secret}
}

type Service struct {
	UnimplementedEnvironServer
	path   string
	secret jwtx.JWTSecretSource
}

func (t *Service) Bind(host grpc.ServiceRegistrar) {
	RegisterEnvironServer(host, t)
}

// Get implements EnvironServer.
func (t *Service) Get(ctx context.Context, _ *GetRequest) (*GetResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	tokens := md.Get("authorization")
	if len(tokens) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}

	var claims jwt.RegisteredClaims
	if err := jwtx.Validate(t.secret, tokens[0], &claims); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}

	if raw, err := os.ReadFile(t.path); err == nil {
		return &GetResponse{Environ: raw}, nil
	} else if !os.IsNotExist(err) {
		return nil, status.Error(codes.Internal, errorsx.Wrap(err, "unable to read environ").Error())
	}

	// no environment file was provided.
	return &GetResponse{}, nil
}
