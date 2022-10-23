package server

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	api "github.com/julieta-311/proglog/api/v1"
	"github.com/julieta-311/proglog/internal/config"
	"github.com/julieta-311/proglog/internal/log"
)

func setupTest(t *testing.T, fn func(*Config)) (
	client api.LogClient,
	cfg *Config,
	teardown func(),
) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	clientTLSConfig, err := config.SetupTLSConfig(
		config.TLSConfig{
			CAFile: config.CAFile,
		})
	require.NoError(t, err)

	clientCreds := credentials.NewTLS(clientTLSConfig)
	cc, err := grpc.Dial(
		l.Addr().String(),
		grpc.WithTransportCredentials(clientCreds))
	require.NoError(t, err)

	client = api.NewLogClient(cc)

	serverTLSConfig, err := config.SetupTLSConfig(
		config.TLSConfig{
			CertFile:      config.ServerCertFile,
			KeyFile:       config.ServerKeyFile,
			CAFile:        config.CAFile,
			ServerAddress: l.Addr().String(),
		})
	require.NoError(t, err)
	serverCreds := credentials.NewTLS(serverTLSConfig)

	dir, err := os.MkdirTemp("", "server-test")
	require.NoError(t, err)

	clog, err := log.NewLog(dir, log.Config{})
	require.NoError(t, err)

	cfg = &Config{
		CommitLog: clog,
	}

	if fn != nil {
		fn(cfg)
	}

	server, err := NewGRPCServer(
		cfg,
		grpc.Creds(serverCreds))
	require.NoError(t, err)

	go func() {
		err := server.Serve(l)
		require.NoError(t, err)
	}()

	return client, cfg, func() {
		server.Stop()
		cc.Close()
		l.Close()

		err = clog.Remove()
		require.NoError(t, err)
	}
}

func TestServer(t *testing.T) {
	for name, fn := range map[string]func(
		s *testing.T,
		client api.LogClient,
		config *Config,
	){
		"produce/consume a message to/from the log succeeds": testProduceConsume,

		"produce/consume stream succeeds": testProduceConsumeStream,

		"consume past log boundary fails": testConsumePastBoundary,
	} {
		t.Run(name, func(s *testing.T) {
			client, config, teardown := setupTest(s, nil)
			defer teardown()

			fn(s, client, config)
		})

	}
}

func testProduceConsume(
	t *testing.T,
	client api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	want := &api.Record{
		Value: []byte("hello world"),
	}
	produce, err := client.Produce(
		ctx,
		&api.ProduceRequest{
			Record: want,
		},
	)
	require.NoError(t, err)

	consume, err := client.Consume(
		ctx,
		&api.ConsumeRequest{
			Offset: produce.Offset,
		},
	)
	require.NoError(t, err)
	require.Equal(t, want.Value, consume.Record.Value)
	require.Equal(t, want.Offset, consume.Record.Offset)
}

func testConsumePastBoundary(
	t *testing.T,
	client api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	produce, err := client.Produce(
		ctx,
		&api.ProduceRequest{
			Record: &api.Record{
				Value: []byte("hola mundo"),
			},
		},
	)
	require.NoError(t, err)

	consume, err := client.Consume(
		ctx,
		&api.ConsumeRequest{
			Offset: produce.Offset + 1,
		},
	)

	if consume != nil {
		t.Fatal("consume not nil")
	}

	got := status.Code(err)
	want := status.Code(
		api.ErrOffsetOutOfRange{}.GRPCStatus().Err(),
	)
	require.Equalf(t, want, got,
		"got err: %v, want: %v", got, want)
}

func testProduceConsumeStream(
	t *testing.T,
	client api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	records := []*api.Record{
		{
			Value:  []byte("first message"),
			Offset: 0,
		},
		{
			Value:  []byte("second message"),
			Offset: 1,
		},
	}

	{
		stream, err := client.ProduceStream(ctx)
		require.NoError(t, err)

		for offset, record := range records {
			err = stream.Send(
				&api.ProduceRequest{
					Record: record,
				},
			)
			require.NoError(t, err)

			res, err := stream.Recv()
			require.NoError(t, err)
			require.Equalf(t, res.Offset, uint64(offset),
				"got offset: %d, want: %d", res.Offset, offset)
		}
	}

	{
		stream, err := client.ConsumeStream(
			ctx,
			&api.ConsumeRequest{
				Offset: 0,
			},
		)
		require.NoError(t, err)

		for i, record := range records {
			want := &api.Record{
				Value:  record.Value,
				Offset: uint64(i),
			}

			res, err := stream.Recv()
			require.NoError(t, err)
			require.Equal(t, want, res.Record)
		}
	}
}
