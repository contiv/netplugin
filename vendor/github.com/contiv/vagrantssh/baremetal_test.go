// +build baremetal

package vagrantssh

import (
	"strings"
	"sync"
	. "testing"

	. "gopkg.in/check.v1"
)

type baremetalTestSuite struct {
	tb Testbed
}

var _ = Suite(&baremetalTestSuite{})

func TestBaremetal(t *T) {
	TestingT(t)
}

func (b *baremetalTestSuite) SetUpSuite(c *C) {
	// This test run inside a vagrant vm and tests connectivity to itself
	hosts := []HostInfo{
		{
			Name:        "self",
			SSHAddr:     "127.0.0.1",
			SSHPort:     "22",
			User:        "vagrant",
			PrivKeyFile: "/vagrant/testdata/insecure_private_key",
		},
		{
			Name:        "self1",
			SSHAddr:     "127.0.0.1",
			SSHPort:     "22",
			User:        "vagrant",
			PrivKeyFile: "/vagrant/testdata/insecure_private_key",
		},
	}
	bm := &Baremetal{}
	c.Assert(bm.Setup(hosts), IsNil)
	b.tb = bm
}

func (b *baremetalTestSuite) TestSetupInvalidArgs(c *C) {
	bm := &Baremetal{}
	c.Assert(bm.Setup(1, "foo"), ErrorMatches, "Unexpected args to Setup.*Expected:.*Received:.*")
}

func (b *baremetalTestSuite) TestRun(c *C) {
	for _, node := range b.tb.GetNodes() {
		c.Assert(node.RunCommand("ls"), IsNil)
	}

	for _, node := range b.tb.GetNodes() {
		c.Assert(node.RunCommand("exit 1"), NotNil)
	}
}

func (b *baremetalTestSuite) TestRunWithOutput(c *C) {
	for _, node := range b.tb.GetNodes() {
		out, err := node.RunCommandWithOutput("whoami")
		c.Assert(err, IsNil)
		c.Assert(strings.TrimSpace(out), Equals, "vagrant")
	}

	for _, node := range b.tb.GetNodes() {
		_, err := node.RunCommandWithOutput("exit 1")
		c.Assert(err, NotNil)
	}
}

func (b *baremetalTestSuite) TestIterateNodes(c *C) {
	mutex := &sync.Mutex{}
	var i int
	c.Assert(b.tb.IterateNodes(func(node TestbedNode) error {
		mutex.Lock()
		i++
		mutex.Unlock()
		return node.RunCommand("exit 0")
	}), IsNil)
	c.Assert(i, Equals, 2)

	i = 0
	c.Assert(b.tb.IterateNodes(func(node TestbedNode) error {
		mutex.Lock()
		i++
		mutex.Unlock()
		return node.RunCommand("exit 1")
	}), NotNil)
	c.Assert(i, Equals, 2)
}
