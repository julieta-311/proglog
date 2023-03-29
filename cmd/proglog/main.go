package main

import (
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/julieta-311/proglog/internal/agent"
	"github.com/julieta-311/proglog/internal/config"
)

func main() {
	cli := &cli{}

	cmd := &cobra.Command{
		Use:     "proglog",
		PreRunE: cli.setupConfig,
		RunE:    cli.run,
	}

	if err := setupFlags(cmd); err != nil {
		log.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func setupFlags(cmd *cobra.Command) error {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	cmd.Flags().String("config-file", "", "Path to config file.")

	cmd.Flags().String(
		"data-dir",
		path.Join(os.TempDir(), "proglog"),
		"Path to the directory where to store log and Raft data.",
	)

	cmd.Flags().String("node-name", hostname, "Unique server ID.")

	cmd.Flags().String("bind-addr", "127.0.0.1:8401", "Address to bind Serf on.")
	cmd.Flags().Int("rpc-port", 8400, "Port for RPC clients (and Raft) connections.")
	cmd.Flags().StringSlice("start-join-addrs", nil, "Serf addresses to join.")
	cmd.Flags().Bool("bootstrap", false, "Bootstrap the cluster.")

	cmd.Flags().String("acl-model-file", "", "Path to ACL model.")
	cmd.Flags().String("acl-policy-file", "", "Path to ACL policy.")

	cmd.Flags().String("peer-tls-cert-file", "", "Path to peer tls cert.")
	cmd.Flags().String("peer-tls-key-file", "", "Path to peer tls key")
	cmd.Flags().String("peer-tls-ca-file", "", "Path to peer certificate authority.")

	return viper.BindPFlags(cmd.Flags())
}

type cli struct {
	cfg cfg
}

type cfg struct {
	agent.Config
	ServerTLSConfig config.TLSConfig
	PeerTLSConfig   config.TLSConfig
}

// setupConfig reads the configuration and prepares the agent's configuration.
// Cobra calls setupConfig before running the command's RunE function.
func (c *cli) setupConfig(cmd *cobra.Command, args []string) error {
	configFile, err := cmd.Flags().GetString("config-file")
	if err != nil {
		return err
	}
	viper.SetConfigFile(configFile)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}

	c.cfg.DataDir = viper.GetString("data-dir")
	c.cfg.NodeName = viper.GetString("node-name")
	c.cfg.BindAddr = viper.GetString("bind-addr")
	c.cfg.RPCPort = viper.GetInt("rpc-port")
	c.cfg.StartJoinAddrs = viper.GetStringSlice("start-join-addrs")
	c.cfg.Bootstrap = viper.GetBool("bootstrap")
	c.cfg.ACLModelFile = viper.GetString("acl-model-file")
	c.cfg.ACLPolicyFile = viper.GetString("acl-policy-file")
	c.cfg.ServerTLSConfig.CertFile = viper.GetString("server-tls-cert-file")
	c.cfg.ServerTLSConfig.KeyFile = viper.GetString("server-tls-key-file")
	c.cfg.ServerTLSConfig.CAFile = viper.GetString("server-tls-ca-file")
	c.cfg.PeerTLSConfig.CertFile = viper.GetString("peer-tls-cert-file")
	c.cfg.PeerTLSConfig.KeyFile = viper.GetString("peer-tls-key-file")
	c.cfg.PeerTLSConfig.CAFile = viper.GetString("peer-tls-ca-file")

	if c.cfg.ServerTLSConfig.CertFile != "" && c.cfg.ServerTLSConfig.KeyFile != "" {
		c.cfg.ServerTLSConfig.Server = true

		var err error
		c.cfg.Config.ServerTLSConfig, err = config.SetupTLSConfig(
			c.cfg.ServerTLSConfig,
		)
		if err != nil {
			return err
		}
	}

	if c.cfg.PeerTLSConfig.CertFile != "" && c.cfg.PeerTLSConfig.KeyFile != "" {
		var err error
		c.cfg.Config.PeerTLSConfig, err = config.SetupTLSConfig(
			c.cfg.PeerTLSConfig,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// run creates the agent, handles signals from the operating system,
// and handles shut down signals gracefully.
func (c *cli) run(cmd *cobra.Command, args []string) error {
	agent, err := agent.New(c.cfg.Config)
	if err != nil {
		return err
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc

	return agent.Shutdown()
}
