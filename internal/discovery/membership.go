package discovery

import (
	"fmt"
	"net"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
	"go.uber.org/zap"
)

// Membership is a Serf wrapper to
// provide discovery and cluster
// membership to services.
type Membership struct {
	Config
	handler Handler
	serf    *serf.Serf
	events  chan serf.Event
	logger  *zap.Logger
}

// New is a factory function to create a Membership.
func New(handler Handler, config Config) (*Membership, error) {
	c := &Membership{
		Config:  config,
		handler: handler,
		logger:  zap.L().Named("membership"),
	}
	if err := c.setupSerf(); err != nil {
		return nil, fmt.Errorf(
			"failed to set membership up: %v",
			err,
		)
	}

	return c, nil
}

// Config holds node configuration.
type Config struct {
	// Nodename acts as the node's unique identifier
	// across the cluster.
	NodeName string

	// BindAddr is used by Serf for gossiping.
	BindAddr string

	// Tags are shared to other nodes, they are used
	// to inform the cluster how to handle the node.
	Tags map[string]string

	// StartJoinAddrs is used to configure how to
	// add new nodes to an existing cluster.
	StartJoinAddrs []string
}

// setufSerf creates and configures a Serf instance and starts
// the eventsHandler() goroutine to handle Serf events.
func (m *Membership) setupSerf() (err error) {
	addr, err := net.ResolveTCPAddr("tcp", m.BindAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve addr: %v", err)
	}

	config := serf.DefaultConfig()
	config.Init()
	config.MemberlistConfig.BindAddr = addr.IP.String()
	config.MemberlistConfig.BindPort = addr.Port

	m.events = make(chan serf.Event)
	config.EventCh = m.events

	config.Tags = m.Tags
	config.NodeName = m.NodeName

	m.serf, err = serf.Create(config)
	if err != nil {
		return fmt.Errorf("failed to instanciate: %v", err)
	}

	go m.eventHandler()
	if m.StartJoinAddrs != nil {
		_, err = m.serf.Join(m.StartJoinAddrs, true)
		if err != nil {
			return err
		}
	}

	return nil
}

// Handler represents a component in a
// service that needs to know when a
// server joins or leaves the cluster.
type Handler interface {
	Join(name, addr string) error
	Leave(name string) error
}

func (m *Membership) eventHandler() {
	for e := range m.events {
		switch e.EventType() {
		case serf.EventMemberJoin:
			for _, member := range e.(serf.MemberEvent).Members {
				if m.isLocal(member) {
					continue
				}
				m.handleJoin(member)
			}
		case serf.EventMemberLeave, serf.EventMemberFailed:
			for _, member := range e.(serf.MemberEvent).Members {
				if m.isLocal(member) {
					continue
				}
				m.handleLeave(member)
			}
		}
	}
}

func (m *Membership) handleJoin(member serf.Member) {
	if err := m.handler.Join(
		member.Name,
		member.Tags["rpc_addr"],
	); err != nil {
		m.logError(err, "failed to join", member)
	}
}

func (m *Membership) handleLeave(member serf.Member) {
	if err := m.handler.Leave(
		member.Name,
	); err != nil {
		m.logError(err, "failed to leave", member)
	}
}

// isLocal returns whether the given member is the local
// member by checking the members' names.
func (m *Membership) isLocal(member serf.Member) bool {
	return m.serf.LocalMember().Name == member.Name
}

// Members returns all members in the cluster.
func (m *Membership) Members() []serf.Member {
	return m.serf.Members()
}

// Leave removes a member from the cluster.
func (m *Membership) Leave() error {
	return m.serf.Leave()
}

func (m *Membership) logError(err error, msg string, member serf.Member) {
	log := m.logger.Error
	if err == raft.ErrNotLeader {
		log = m.logger.Debug
	}

	log(
		msg,
		zap.Error(err),
		zap.String("name", member.Name),
		zap.String("rpc_addr", member.Tags["rpc_addr"]),
	)
}
