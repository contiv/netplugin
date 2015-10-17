<h1>Service discovery using Skydns</h1>

<h4>Starting SkyDNS in a container</h4>
Start SkyDNS in a container and attach it to the bridge network to allow backend communication to etcd servers

`docker run -d --publish-service=skydns.bridge --name skydns skynetservices/skydns -addr 0.0.0.0:53 -machines http://172.17.42.1:4001 -nameservers 8.8.8.8:53 -domain skydns.local`

<h4>Create new network and attach SkyDNS to the new network</h4>
```
contivctl network create -public yes -encap vxlan -subnet 20.1.1.0/24 -defaultGw 20.1.1.254 web
docker service publish skydns.web
docker service attach <skydns-container-id> skydns.web
```

<h4>Launch services in the network</h4>
```
docker run -itd --publish-service=web1.web --dns=20.1.1.1 --name=web1 --hostname=web1 tracert /bin/bash
docker run -itd --publish-service=web2.web --dns=20.1.1.1 --name=web2 --hostname=web2 tracert /bin/bash
docker run -itd --publish-service=web3.web --dns=20.1.1.1 --name=web3 --hostname=web3 tracert /bin/bash
```
<h4>DNS query for SRV records lists all services</h4>
```
root@web1:/# dig web.skydns.local SRV
...
;; QUESTION SECTION:
;web.skydns.local.              IN      SRV

;; ANSWER SECTION:
web.skydns.local.       3600    IN      SRV     10 25 0 web3.web.skydns.local.
web.skydns.local.       3600    IN      SRV     10 25 0 skydns.web.skydns.local.
web.skydns.local.       3600    IN      SRV     10 25 0 web1.web.skydns.local.
web.skydns.local.       3600    IN      SRV     10 25 0 web2.web.skydns.local.

;; ADDITIONAL SECTION:
web3.web.skydns.local.  3600    IN      A       20.1.1.5
skydns.web.skydns.local. 3600   IN      A       20.1.1.2
web1.web.skydns.local.  3600    IN      A       20.1.1.3
web2.web.skydns.local.  3600    IN      A       20.1.1.4
```
<h4>Stop a container</h4>
```
vagrant@netplugin-node1:/opt/gopath/src/github.com/contiv/netplugin$ docker stop web1
web1
```
<h4>DNS query is updated to reflect the container that has left</h4>
```
vagrant@netplugin-node1:/opt/gopath/src/github.com/contiv/netplugin$ docker exec -it web2 bash
root@web2:/# dig web.skydns.local SRV
...
;; QUESTION SECTION:
;web.skydns.local.              IN      SRV

;; ANSWER SECTION:
web.skydns.local.       3600    IN      SRV     10 33 0 web2.web.skydns.local.
web.skydns.local.       3600    IN      SRV     10 33 0 web3.web.skydns.local.
web.skydns.local.       3600    IN      SRV     10 33 0 skydns.web.skydns.local.

;; ADDITIONAL SECTION:
web2.web.skydns.local.  3600    IN      A       20.1.1.4
web3.web.skydns.local.  3600    IN      A       20.1.1.5
skydns.web.skydns.local. 3600   IN      A       20.1.1.2
...
```
