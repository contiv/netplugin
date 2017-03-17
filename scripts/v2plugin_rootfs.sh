#!/bin/bash
# Script to create the docker v2 plugin
# run this script from contiv/netplugin directory

echo "Creating rootfs for v2plugin ", ${CONTIV_V2PLUGIN_NAME}
cat install/v2plugin/config.template | grep -v "##" > install/v2plugin/config.json
sed -i "s%PluginName%${CONTIV_V2PLUGIN_NAME}%" install/v2plugin/config.json
cp bin/netplugin bin/netmaster bin/netctl install/v2plugin
docker build -t contivrootfs install/v2plugin
id=$(docker create contivrootfs true)
mkdir -p install/v2plugin/rootfs
sudo docker export "${id}" | sudo tar -x -C install/v2plugin/rootfs
docker rm -vf "${id}"
docker rmi contivrootfs
rm install/v2plugin/netplugin install/v2plugin/netmaster install/v2plugin/netctl
