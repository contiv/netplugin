#/!/bin/bash

# Script run on the Jenkins Node after the build(tests) are completed.
# Forcibly cleans up _ALL_ VirtualBox VMs, not just those created during
# the Jenkins run.

# Doing each command with "|| true" so that even if a command fails, it won't
# cause Jenkins to mark the build as failed.

set -e

echo "Starting cleanup."
echo "Existing VMs:"
vboxmanage list vms
echo "------------"

cd $WORKSPACE/src/github.com/contiv/netplugin
vagrant destroy -f || true

rm -rf /home/admin/VirtualBox\ VMs/* || true
rm -rf .vagrant/* || true
rm -f *.vdi || true

for f in $(vboxmanage list vms | awk {'print $2'} | cut -d'{' -f2 | cut -d'}' -f1); do
	echo $f
	vboxmanage controlvm $f poweroff || true
	sleep 5
	vboxmanage unregistervm --delete $f || true
done

echo "Cleanup finished."
echo "any VMs still left?"
vboxmanage list vms
echo "------------"
