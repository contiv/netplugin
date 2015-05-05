#Powerstrip based integration

Netplugin can now be used with docker using powerstrip.

##Trying it out
- Start the demo vm
```
make demo
ssh netplugin-node1
sudo -s
source /etc/profile.d/envvar.sh
```
- Start netplugin
```
netplugin -host-label=host1 -native-integration &> /tmp/netplugin.log &
```
- Start the powerstrip adapter
```
cd $GOSRC/github.com/contiv/netplugin
docker build -t netplugin/pslibnet mgmtfn/pslibnet
docker run -d --name pslibnet --expose 80 netplugin/pslibnet --host-label=host1 --etcd-url=http://192.168.2.10:2379
```
- Start powerstrip
```
cd
mkdir demo
cat > demo/adapters.yml <<EOF
version: 1
endpoints:
  "POST /*/containers/create":
    pre: [pslibnet]
    post: [pslibnet]
  "POST /*/containers/*/start":
    pre: [pslibnet]
    post: [pslibnet]
  "POST /*/containers/*/stop":
    pre: [pslibnet]
  "POST /*/containers/*/delete":
    pre: [pslibnet]
adapters:
  pslibnet: http://pslibnet/adapter/
EOF
docker run -d --name powerstrip -v /var/run/docker.sock:/var/run/docker.sock -v $PWD/demo/adapters.yml:/etc/powerstrip/adapters.yml --link pslibnet:pslibnet -p 2375:2375 clusterhq/powerstrip:v0.0.1
```
- Create a network
```
cd $GOSRC/github.com/contiv/netplugin
netdcli -cfg examples/late_bindings/powerstrip_demo_vlan_nets.json
```
- Run container1
```
ssh netplugin-node1
DOCKER_HOST=localhost:2375 docker run -it --name=myContainer1 --label netid=orange --label tenantid=tenant-one ubuntu bash
ip addr show
```
- Run container2
```
ssh netplugin-node1
DOCKER_HOST=localhost:2375 docker run -it --name=myContainer2 --label netid=orange --label tenantid=tenant-one ubuntu bash
ping <ip-address of container myContainer1>
```
