package discovery_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
	"github.com/travisjeffery/go-dynaport"

	. "github.com/julieta-311/proglog/internal/discovery"
)

func TestMembership(t *testing.T) {
	m, handler := setupMember(t, nil)
	m, _ = setupMember(t, m)
	m, _ = setupMember(t, m)

	require.Eventually(t, func() bool {
		return len(handler.joins) == 2 &&
			len(m[0].Members()) == 3 &&
			len(handler.leaves) == 0
	},
		3*time.Second,
		250*time.Millisecond,
	)

	require.NoError(t, m[2].Leave())

	require.Eventually(t, func() bool {
		return len(handler.joins) == 2 &&
			len(m[0].Members()) == 3 &&
			m[0].Members()[2].Status == serf.StatusLeft &&
			len(handler.leaves) == 1
	},
		3*time.Second,
		250*time.Millisecond,
	)

	require.Equal(t, fmt.Sprintf("%d", 2), <-handler.leaves)
}

// setupMember sets up a new member under a free port
// using the member's length as the node name so that
// names are unique. The member's lenth is also an
// indicator of whether this member is the cluster's
// initial member or if there's a cluster to join.
func setupMember(t *testing.T, members []*Membership) (
	[]*Membership, *handler,
) {
	id := len(members)
	ports := dynaport.Get(1)
	addr := fmt.Sprintf("%s:%d", "127.0.0.1", ports[0])
	tags := map[string]string{
		"rpc_addr": addr,
	}

	c := Config{
		NodeName: fmt.Sprintf("%d", id),
		BindAddr: addr,
		Tags:     tags,
	}

	h := &handler{}
	if len(members) == 0 {
		h.joins = make(chan map[string]string, 3)
		h.leaves = make(chan string, 3)
	} else {
		c.StartJoinAddrs = []string{
			members[0].BindAddr,
		}
	}

	m, err := New(h, c)
	require.NoError(t, err)

	members = append(members, m)
	return members, h
}

// handler tracks how many times a
// membership calls the handler's
// join and leave mehtods.
type handler struct {
	joins  chan map[string]string
	leaves chan string
}

// Join mocks a node joining the cluster.
func (h *handler) Join(id, addr string) error {
	if h.joins != nil {
		h.joins <- map[string]string{
			"id":   id,
			"addr": addr,
		}
	}

	return nil
}

// Leave mocks a node leaving the cluster.
func (h *handler) Leave(id string) error {
	if h.leaves != nil {
		h.leaves <- id
	}

	return nil
}
