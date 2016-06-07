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
	
