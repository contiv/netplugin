## IPv6 in netplugin
The current implementation supports dual stack with IPv4 and IPv6 interfaces for containers, as Docker Swarm APIs do not support IPv6-only containers.

## Configuring IPv6 network 
When configuring a network, you can specify an optional IPv6 subnet pool to allocate IPv6 address for the containers.

```
netctl net create contiv-net --subnet=20.1.1.0/24 --subnetv6=2001::/100
```

## IP Address allocation
The containers created will get IPv4 and IPv6 address allocated from the corresponding subnet range.
```	
[vagrant@netplugin-node1 netplugin]$ docker network inspect contiv-net
[
    {
        "Name": "contiv-net",
        "Id": "6c21fa082f4f456730928d076b19148e6db1d88d90fe8f2c0760223af51f4e31",
        "Scope": "global",
        "Driver": "netplugin",
        "IPAM": {
            "Driver": "netplugin",
            "Config": [
                {
                    "Subnet": "20.1.1.0/24"
                },
                {
                    "Subnet": "2001::/100"
                }
            ]
        },
        "Containers": {
            "455a80938abcaf4a534a08783ff44e10b2a1a5afb3c2690d474b19ed670b3d7a": {
                "EndpointID": "d8fcf4dc8d79841073655698ebc692ffe20251abc1c2577cdc6924fb53d7ef62",
                "MacAddress": "02:02:14:01:01:02",
                "IPv4Address": "20.1.1.2/24",
                "IPv6Address": "2001::2/100"
            },
            "cb30c9001c118f4ae39b6fda14f6c26c9ccbdcd1110eabded179ce47a7aa63c9": {
                "EndpointID": "06e11d75f0bada4b1e1dad848363b3fea323eb3ac175cde0569b3e7f25dff3a0",
                "MacAddress": "02:02:14:01:01:03",
                "IPv4Address": "20.1.1.3/24",
                "IPv6Address": "2001::3/100"
            }
        },
        "Options": {
            "encap": "vxlan",
            "pkt-tag": "1",
            "tenant": "default"
        }
    }
]
```
	
## Container IPv6 interface

Container 'web' in Node1
```
[vagrant@netplugin-node1 netplugin]$ docker exec -it web /bin/sh
/ # ip addr show dev eth0
13: eth0@if12: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1450 qdisc noqueue state UP 
    link/ether 02:02:14:01:01:03 brd ff:ff:ff:ff:ff:ff
    inet 20.1.1.3/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 2001::3/100 scope global 
       valid_lft forever preferred_lft forever
    inet6 fe80::2:14ff:fe01:103/64 scope link 
       valid_lft forever preferred_lft forever
/ # 
```

Container 'db' in Node2
```
[vagrant@netplugin-node2 ~]$ docker exec -it db /bin/sh
/ # ip addr show dev eth0
9: eth0@if8: <BROADCAST,MULTICAST,UP,LOWER_UP,M-DOWN> mtu 1450 qdisc noqueue state UP 
    link/ether 02:02:14:01:01:04 brd ff:ff:ff:ff:ff:ff
    inet 20.1.1.4/24 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 2001::4/100 scope global 
       valid_lft forever preferred_lft forever
    inet6 fe80::2:14ff:fe01:104/64 scope link 
       valid_lft forever preferred_lft forever
/ # 
```	

## Ping6 between containers
```
/ # ping6 2001::4 -I 2001::3 -c 3
PING 2001::4 (2001::4) from 2001::3: 56 data bytes
64 bytes from 2001::4: seq=0 ttl=64 time=1.689 ms
64 bytes from 2001::4: seq=1 ttl=64 time=2.437 ms
64 bytes from 2001::4: seq=2 ttl=64 time=1.526 ms

--- 2001::4 ping statistics ---
3 packets transmitted, 3 packets received, 0% packet loss
round-trip min/avg/max = 1.526/1.884/2.437 ms
/ # 
```
