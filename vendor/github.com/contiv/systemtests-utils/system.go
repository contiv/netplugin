package utils

import (
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/contiv/vagrantssh"
)

// StopEtcd stops etcd on a specific host
func StopEtcd(nodes []vagrantssh.TestbedNode) error {
	for _, node := range nodes {
		log.Infof("Stopping etcd on node %s", node.GetName())

		if err := node.RunCommand("sudo systemctl stop etcd"); err != nil {
			return err
		}

		times := 10

		for {
			if err := node.RunCommand("etcdctl member list"); err != nil {
				break
			}

			times--

			if times < 0 {
				return fmt.Errorf("Timed out stopping etcd on %s", node.GetName())
			}

			time.Sleep(100 * time.Millisecond)
		}
	}
	return nil
}

func ClearEtcd(node vagrantssh.TestbedNode) {
	log.Infof("Clearing etcd data")
	node.RunCommand(`for i in $(etcdctl ls /); do etcdctl rm --recursive "$i"; done`)
}

// StartEtcd starts etcd on a specific host.
func StartEtcd(nodes []vagrantssh.TestbedNode) error {
	for _, node := range nodes {
		log.Infof("Starting etcd on %s", node.GetName())
		times := 10

		for {
			// the error is not checked here because we will not successfully start
			// etcd the second time we try, but want to retry if the first one fails.
			node.RunCommand("sudo systemctl start etcd")

			time.Sleep(1 * time.Second)

			if err := node.RunCommand("etcdctl member list"); err == nil {
				break
			}

			times--

			if times < 0 {
				return fmt.Errorf("Timed out starting etcd on %s", node.GetName())
			}
		}
	}

	return nil
}

//ServiceStart starts a systemd service unit
func ServiceStart(n vagrantssh.TestbedNode, srv string) (string, error) {
	return n.RunCommandWithOutput(fmt.Sprintf("sudo systemctl start %s", srv))
}

//ServiceStatus queries and returns status result of systemd service unit
func ServiceStatus(n vagrantssh.TestbedNode, srv string) (string, error) {
	return n.RunCommandWithOutput(fmt.Sprintf("systemctl status %s", srv))
}

// WaitForDone polls for checkDoneFn function to return true up until specified timeout
func WaitForDone(doneFn func() (string, bool), tickDur time.Duration, timeoutDur time.Duration, timeoutMsg string) (string, error) {
	tick := time.Tick(tickDur)
	timeout := time.After(timeoutDur)
	doneCount := 0
	for {
		select {
		case <-tick:
			if ctxt, done := doneFn(); done {
				doneCount++
				// add some resilliency to poll inorder to avoid false positives,
				// while polling more frequently
				if doneCount == 2 {
					// end poll
					return ctxt, nil
				}
			}
			//continue polling
		case <-timeout:
			ctxt, done := doneFn()
			if !done {
				return ctxt, fmt.Errorf("wait timeout. Msg: %s", timeoutMsg)
			}
			return ctxt, nil
		}
	}
}

// ServiceActionAndWaitForState call the specified action function on a service unit and
// waits for the unit to reach expected state up until the timeout
func ServiceActionAndWaitForState(n vagrantssh.TestbedNode, srv string, timeoutSec int, state string,
	action func(vagrantssh.TestbedNode, string) (string, error)) (string, error) {
	out, err := action(n, srv)
	if err != nil {
		return out, err
	}

	return WaitForDone(func() (string, bool) {
		out, err := ServiceStatus(n, srv)
		if err == nil && strings.Contains(out, "Active: "+state) {
			return out, true
		}
		return out, false
	}, 2*time.Second, time.Duration(timeoutSec)*time.Second, fmt.Sprintf("it seems that service %q is not %q", srv, state))
}

//ServiceStartAndWaitForUp starts a systemd service unit and waits for it to be up
func ServiceStartAndWaitForUp(n vagrantssh.TestbedNode, srv string, timeoutSec int) (string, error) {
	return ServiceActionAndWaitForState(n, srv, timeoutSec, "active", ServiceStart)
}

//ServiceStop stops a systemd service unit
func ServiceStop(n vagrantssh.TestbedNode, srv string) (string, error) {
	return n.RunCommandWithOutput(fmt.Sprintf("sudo systemctl stop %s", srv))
}

//ServiceRestart restarts a systemd service unit
func ServiceRestart(n vagrantssh.TestbedNode, srv string) (string, error) {
	return n.RunCommandWithOutput(fmt.Sprintf("sudo systemctl restart %s", srv))
}

//ServiceRestartAndWaitForUp starts a systemd service unit and waits for it to be up
func ServiceRestartAndWaitForUp(n vagrantssh.TestbedNode, srv string, timeoutSec int) (string, error) {
	return ServiceActionAndWaitForState(n, srv, timeoutSec, "active", ServiceRestart)
}

//ServiceLogs queries and returns upto maxLogLines lines from systemd service unit logs
func ServiceLogs(n vagrantssh.TestbedNode, srv string, maxLogLines int) (string, error) {
	return n.RunCommandWithOutput(fmt.Sprintf("sudo systemctl status -ln%d %s", maxLogLines, srv))
}
