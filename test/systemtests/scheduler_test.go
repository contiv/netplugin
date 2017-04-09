package systemtests

/* An interface defined for testing different schedulers.
   Support for kubernetes, swarm and no scheduler scenarios.
   Scalable for other schedulers also, if required.
*/
type systemTestScheduler interface {
	runContainer(spec containerSpec) (*container, error)
	stop(c *container) error
	start(c *container) error
	startNetmaster(args string) error
	stopNetmaster() error
	stopNetplugin() error
	startNetplugin(args string) error
	cleanupContainers() error
	checkNoConnection(c *container, ipaddr, protocol string, port int) error
	checkConnection(c *container, ipaddr, protocol string, port int) error
	startListener(c *container, port int, protocol string) error
	rm(c *container) error
	getIPAddr(c *container, dev string) (string, error)
	getMACAddr(c *container, dev string) (string, error)
	checkPing(c *container, ipaddr string) error
	checkPing6(c *container, ipv6addr string) error
	checkPingFailure(c *container, ipaddr string) error
	checkPing6Failure(c *container, ipv6addr string) error
	cleanupSlave()
	cleanupMaster()
	runCommandUntilNoNetpluginError() error
	runCommandUntilNoNetmasterError() error
	rotateNetmasterLog() error
	rotateNetpluginLog() error
	getIPv6Addr(c *container, dev string) (string, error)
	checkForNetpluginErrors() error
	rotateLog(prefix string) error
	checkConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error
	checkNoConnectionRetry(c *container, ipaddr, protocol string, port, delay, retries int) error
	checkPingWithCount(c *container, ipaddr string, count int) error
	checkPing6WithCount(c *container, ipaddr string, count int) error
	checkSchedulerNetworkCreated(nwName string, expectedOp bool) error
	checkSchedulerNetworkOnNodeCreated(nwName []string, n *node) error
	waitForListeners() error
	verifyVTEPs(expVTEPS map[string]bool) (string, error)
	verifyAgents(expVTEPS map[string]bool) (string, error)
	verifyEPs(epList []string) (string, error)
	reloadNode(n *node) error
	getMasterIP() (string, error)
	startIperfServer(containers *container) error
	startIperfClient(containers *container, ip, limit string, isErr bool) error
	tcFilterShow(bw string) error
	verifyUplinkState(n *node, uplinks []string) error
}
