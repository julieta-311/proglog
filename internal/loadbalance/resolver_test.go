package loadbalance_test

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"

	"github.com/julieta-311/proglog/internal/config"
	"github.com/julieta-311/proglog/internal/loadbalance"
	"github.com/julieta-311/proglog/internal/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/serviceconfig"

	api "github.com/julieta-311/proglog/api/v1"
)

func TestResolver(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	tlsConfig, err := config.SetupTLSConfig(
		config.TLSConfig{
			CertFile:      config.ServerCertFile,
			KeyFile:       config.ServerKeyFile,
			CAFile:        config.CAFile,
			Server:        true,
			ServerAddress: "127.0.0.1",
		})
	require.NoError(t, err)

	serverCreds := credentials.NewTLS(tlsConfig)

	srv, err := server.NewGRPCServer(
		&server.Config{
			GetServerer: &getServers{},
		},
		grpc.Creds(serverCreds),
	)
	require.NoError(t, err)

	go func() {
		if err := srv.Serve(l); err != nil {
			fmt.Fprintf(os.Stderr, "server failed: %v", err)
		}
	}()

	conn := &clientConn{}
	tlsConfig, err = config.SetupTLSConfig(
		config.TLSConfig{
			CertFile:      config.RootClientCertFile,
			KeyFile:       config.RootClientKeyFile,
			CAFile:        config.CAFile,
			Server:        false,
			ServerAddress: "127.0.0.1",
		})
	require.NoError(t, err)

	clientCreds := credentials.NewTLS(tlsConfig)
	opts := resolver.BuildOptions{
		DialCreds: clientCreds,
	}

	u, err := url.Parse(fmt.Sprintf("https://%v", l.Addr().String()))
	require.NoError(t, err)
	require.NotNil(t, u)

	r := &loadbalance.Resolver{}
	_, err = r.Build(
		resolver.Target{
			URL: *u,
		},
		conn,
		opts,
	)
	require.NoError(t, err)

	wantState := resolver.State{
		Addresses: []resolver.Address{
			{
				Addr:       "localhost:9001",
				Attributes: attributes.New("is_leader", true),
			},
			{
				Addr:       "localhost:9002",
				Attributes: attributes.New("is_leader", false),
			},
		},
	}
	require.Equal(t, wantState, conn.state)

	conn.state.Addresses = nil
	r.ResolveNow(resolver.ResolveNowOptions{})
	require.Equal(t, wantState, conn.state)
}

type getServers struct{}

// GetServers mocks the GetServer method of the GetServerer interface.
func (s *getServers) GetServers() ([]*api.Server, error) {
	return []*api.Server{
		{
			Id:       "leader",
			RpcAddr:  "localhost:9001",
			IsLeader: true,
		},
		{
			Id:      "follower",
			RpcAddr: "localhost:9002",
		},
	}, nil
}

// clientConn implements the resolver.ClientConn interface.
type clientConn struct {
	resolver.ClientConn
	state resolver.State
}

// UpdateState mocks the UpdateState method for resolver.ClientConn.
func (c *clientConn) UpdateState(state resolver.State) error {
	c.state = state
	return nil
}

// ReportError mocks the ReportError method for
// the resolver.ClientConn interface.
func (c *clientConn) ReportError(err error) {}

// NewAddress mocks the NewAddress method for the
// resolver.ClientConn interface.
func (c *clientConn) NewAddress(addrs []resolver.Address) {}

// NewService mocks the NewService method for the
// resolver.ClientConn interface.
func (c *clientConn) NewService(config string) {}

// ParseServiceConfig mocks the ParseServiceConfig
// method for the resolver.ClientConn interface.
func (c *clientConn) ParseServiceConfig(
	config string,
) *serviceconfig.ParseResult {
	return nil
}
