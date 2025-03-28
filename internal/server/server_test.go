package server

import (
	"context"
	"flag"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opencensus.io/examples/exporter"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	api "github.com/julieta-311/proglog/api/v1"
	"github.com/julieta-311/proglog/internal/auth"
	"github.com/julieta-311/proglog/internal/config"
	"github.com/julieta-311/proglog/internal/log"
)

var debug = flag.Bool("debug", false, "Enable observability for debugging.")

func TestMain(m *testing.M) {
	flag.Parse()
	if *debug {
		logger, err := zap.NewDevelopment()
		if err != nil {
			panic(err)
		}

		zap.ReplaceGlobals(logger)
	}

	os.Exit(m.Run())
}

func setupTest(t *testing.T, fn func(*Config)) (
	rootClient api.LogClient,
	nobodyClient api.LogClient,
	cfg *Config,
	teardown func(),
) {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	newClient := func(crtPath, keyPath string) (
		*grpc.ClientConn,
		api.LogClient,
		[]grpc.DialOption,
	) {
		tlsConfig, err := config.SetupTLSConfig(
			config.TLSConfig{
				CertFile: crtPath,
				KeyFile:  keyPath,
				CAFile:   config.CAFile,
				Server:   false,
			})
		require.NoError(t, err)

		tlsCreds := credentials.NewTLS(tlsConfig)
		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(tlsCreds),
		}
		conn, err := grpc.Dial(l.Addr().String(), opts...)
		require.NoError(t, err)

		client := api.NewLogClient(conn)
		return conn, client, opts
	}

	var rootConn *grpc.ClientConn
	rootConn, rootClient, _ = newClient(
		config.RootClientCertFile,
		config.RootClientKeyFile,
	)

	var nobodyConn *grpc.ClientConn
	nobodyConn, nobodyClient, _ = newClient(
		config.NobodyClientCertFile,
		config.NobodyClientKeyFile,
	)

	serverTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
		CertFile: config.ServerCertFile,
		KeyFile:  config.ServerKeyFile,
		CAFile:   config.CAFile,
		Server:   true,
	})
	require.NoError(t, err)

	serverCreds := credentials.NewTLS(serverTLSConfig)

	dir, err := os.MkdirTemp("", "server-test")
	require.NoError(t, err)
	defer func() { require.NoError(t, os.RemoveAll(dir)) }()

	clog, err := log.NewLog(dir, log.Config{})
	require.NoError(t, err)

	authorizer := auth.New(
		config.ACLModelFile,
		config.ACLPolicyFile,
	)

	var telemetryExporter *exporter.LogExporter
	if *debug {
		metricsLogFile, err := os.CreateTemp("", "metrics-*.log")
		require.NoError(t, err)
		t.Logf("metrics log file: %s", metricsLogFile.Name())

		tracesLogFile, err := os.CreateTemp("", "traces-*.log")
		require.NoError(t, err)
		t.Logf("metrics log file: %s", tracesLogFile.Name())

		telemetryExporter, err = exporter.NewLogExporter(
			exporter.Options{
				MetricsLogFile:    metricsLogFile.Name(),
				TracesLogFile:     tracesLogFile.Name(),
				ReportingInterval: time.Second,
			},
		)
		require.NoError(t, err)

		err = telemetryExporter.Start()
		require.NoError(t, err)
	}

	cfg = &Config{
		CommitLog:  clog,
		Authorizer: authorizer,
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

	return rootClient, nobodyClient, cfg, func() {
		server.Stop()
		if err := rootConn.Close(); err != nil {
			t.Logf("closing root conn: %v", err)
		}
		if err := nobodyConn.Close(); err != nil {
			t.Logf("closing nobody conn: %v", err)
		}
		if err := l.Close(); err != nil {
			t.Logf("failed to close: %v", err)
		}

		require.NoError(t, clog.Remove())

		if telemetryExporter != nil {
			time.Sleep(1500 * time.Millisecond)
			telemetryExporter.Stop()
			telemetryExporter.Close()
		}
	}
}

func TestServer(t *testing.T) {
	for scenario, fn := range map[string]func(
		s *testing.T,
		rootClient api.LogClient,
		nobodyClient api.LogClient,
		config *Config,
	){
		"produce/consume a message to/from the log succeeds": testProduceConsume,

		"produce/consume stream succeeds": testProduceConsumeStream,

		"consume past log boundary fails": testConsumePastBoundary,

		"unauthorized fails": testUnauthorized,
	} {
		t.Run(scenario, func(s *testing.T) {
			rootClient,
				nobodyClient,
				config,
				teardown := setupTest(s, nil)
			defer teardown()

			fn(s, rootClient, nobodyClient, config)
		})

	}
}

func testProduceConsume(
	t *testing.T,
	client api.LogClient,
	_ api.LogClient,
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
	_ api.LogClient,
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
	_ api.LogClient,
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

func testUnauthorized(
	t *testing.T,
	_,
	client api.LogClient,
	config *Config,
) {
	ctx := context.Background()

	produce, err := client.Produce(
		ctx,
		&api.ProduceRequest{
			Record: &api.Record{
				Value: []byte("hello world"),
			},
		},
	)
	require.Nil(t, produce)

	gotCode := status.Code(err)
	wantCode := codes.PermissionDenied
	require.Equal(t, wantCode, gotCode)

	consume, err := client.Consume(
		ctx,
		&api.ConsumeRequest{
			Offset: 0,
		},
	)
	require.Nil(t, consume)

	gotCode = status.Code(err)
	wantCode = codes.PermissionDenied
	require.Equal(t, wantCode, gotCode)
}
