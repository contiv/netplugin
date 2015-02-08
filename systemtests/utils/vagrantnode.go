package utils

type TestbedNode interface {
	RunCommand(cmd string) (err error)
	RunCommandWithOutput(cmd string) (output string, err error)
}

type VagrantNode struct {
	Name    string
	NodeNum int
}

func (n *VagrantNode) RunCommand(cmd string) error {
	vCmd := &VagrantCommand{ContivNodes: n.NodeNum}
	return vCmd.Run("ssh", n.Name, "-c", cmd)
}

func (n *VagrantNode) RunCommandWithOutput(cmd string) (string, error) {
	vCmd := &VagrantCommand{ContivNodes: n.NodeNum}
	output, err := vCmd.RunWithOutput("ssh", n.Name, "-c", cmd)
	return string(output), err
}
