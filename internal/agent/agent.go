package agent

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/soheilhy/cmux"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/julieta-311/proglog/internal/auth"
	"github.com/julieta-311/proglog/internal/discovery"
	"github.com/julieta-311/proglog/internal/log"
	"github.com/julieta-311/proglog/internal/server"
)

// Agent is meant to run on every service
// instance setting up and connecting all
// the different components, that is log,
// server, membership and replicator.
type Agent struct {
	Config

	mux        cmux.CMux
	log        *log.DistributedLog
	server     *grpc.Server
	membership *discovery.Membership

	shutdown     bool
	shutdowns    chan struct{}
	shutdownLock sync.Mutex
}

// Config comprises the config
// needed to set up the components
// managed by the agent.
type Config struct {
	ServerTLSConfig *tls.Config
	PeerTLSConfig   *tls.Config
	DataDir         string
	BindAddr        string
	RPCPort         int
	NodeName        string
	StartJoinAddrs  []string
	ACLModelFile    string
	ACLPolicyFile   string
	Bootstrap       bool
}

// RPCAddr extracts the RPC address from the config.
func (c Config) RPCAddr() (string, error) {
	host, _, err := net.SplitHostPort(c.BindAddr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", host, c.RPCPort), nil
}

// New creates an Agent and runs a set of
// methods to set up and run the agent's
// components.
func New(config Config) (*Agent, error) {
	a := &Agent{
		Config:    config,
		shutdowns: make(chan struct{}),
	}

	setup := []func() error{
		a.setupLogger,
		a.setupMux,
		a.setupLog,
		a.setupServer,
		a.setupMembership,
	}
	for _, fn := range setup {
		if err := fn(); err != nil {
			return nil, err
		}
	}

	go func() {
		if err := a.serve(); err != nil {
			return
		}
	}()

	return a, nil
}

func (a *Agent) setupLogger() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}

	zap.ReplaceGlobals(logger)

	return nil
}

func (a *Agent) setupMux() error {
	addr, err := net.ResolveTCPAddr("tcp", a.BindAddr)
	if err != nil {
		return err
	}

	rpcAddr := fmt.Sprintf("%s:%d", addr.IP.String(), a.RPCPort)
	ln, err := net.Listen("tcp", rpcAddr)
	if err != nil {
		return err
	}

	a.mux = cmux.New(ln)
	return nil
}

func (a *Agent) setupLog() error {
	raftLn := a.mux.Match(func(reader io.Reader) bool {
		b := make([]byte, 1)
		if _, err := reader.Read(b); err != nil {
			return false
		}

		return bytes.Equal(b, []byte{byte(log.RaftRPC)})
	})

	logConfig := log.Config{}
	logConfig.Raft.StreamLayer = log.NewStreamLayer(
		raftLn,
		a.ServerTLSConfig,
		a.PeerTLSConfig,
	)

	rpcAddr, err := a.RPCAddr()
	if err != nil {
		return err
	}
	logConfig.Raft.BindAddr = rpcAddr

	logConfig.Raft.LocalID = raft.ServerID(
		a.NodeName,
	)
	logConfig.Raft.Bootstrap = a.Bootstrap

	a.log, err = log.NewDistributedLog(
		a.DataDir,
		logConfig,
	)
	if err != nil {
		return err
	}

	if a.Bootstrap {
		err = a.log.WaitForLeader(3 * time.Second)
	}
	return err
}

func (a *Agent) setupServer() error {
	authorizer := auth.New(
		a.ACLModelFile,
		a.ACLPolicyFile,
	)

	serverConfig := &server.Config{
		CommitLog:   a.log,
		Authorizer:  authorizer,
		GetServerer: a.log,
	}

	var opts []grpc.ServerOption
	if a.ServerTLSConfig != nil {
		creds := credentials.NewTLS(
			a.ServerTLSConfig,
		)
		opts = append(opts, grpc.Creds(creds))
	}

	var err error
	a.server, err = server.NewGRPCServer(serverConfig, opts...)
	if err != nil {
		return err
	}

	grpcLn := a.mux.Match(cmux.Any())
	go func() {
		if err := a.server.Serve(grpcLn); err != nil {
			_ = a.Shutdown()
		}
	}()

	return err
}

// setupMembership sets up a system to communicate to the
// distributed log when a server joins or leaves the
// cluster so it can handle replication.
func (a *Agent) setupMembership() error {
	rpcAddr, err := a.RPCAddr()
	if err != nil {
		return err
	}

	a.membership, err = discovery.New(a.log, discovery.Config{
		NodeName: a.NodeName,
		BindAddr: a.BindAddr,
		Tags: map[string]string{
			"rpc_addr": rpcAddr,
		},
		StartJoinAddrs: a.StartJoinAddrs,
	})

	return err
}

// Shutdown ensures the agent will shut down
// once, even if Shutdown is called multiple
// times, and that it shuts down the agent
// components.
func (a *Agent) Shutdown() error {
	a.shutdownLock.Lock()
	defer a.shutdownLock.Unlock()

	if a.shutdown {
		return nil
	}

	a.shutdown = true
	close(a.shutdowns)

	shutdown := []func() error{
		a.membership.Leave,
		func() error {
			a.server.GracefulStop()
			return nil
		},
		a.log.Close,
	}
	for _, fn := range shutdown {
		if err := fn(); err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) serve() error {
	if err := a.mux.Serve(); err != nil {
		_ = a.Shutdown()
		return err
	}

	return nil
}
