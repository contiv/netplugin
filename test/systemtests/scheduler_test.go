package systemtests

type systemTestScheduler interface {
	runContainer(spec containerSpec) (*container, error)
	stop(c *container) error
	start(c *container) error
	startNetmaster() error
	stopNetmaster() error
	stopNetplugin() error
	startNetplugin(args string) error
	cleanupContainers() error
	checkNoConnection(c *container, ipaddr, protocol string, port int) error
	checkConnection(c *container, ipaddr, protocol string, port int) error
	startListener(c *container, port int, protocol string) error
	rm(c *container) error
	getIPAddr(c *container, dev string) (string, error)
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
	waitForListeners() error
	verifyVTEPs(expVTEPS map[string]bool) (string, error)
	verifyEPs(epList []string) (string, error)
	reloadNode(n *node) error
	getMasterIP() (string, error)
}
