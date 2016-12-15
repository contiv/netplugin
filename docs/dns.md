<h1>DNS (domain name server)</h1>

* netplugin implements inline DNS in datapath.
* There are no configurations specific to DNS

Inline DNS caches the following information
 
 * Load balancer service names
 * Endpoint group names
 * container names

<h4>Packetflow</h4>

 * netplugin data path receives DNS msg from container
 * netplugin looks up the name in local cache
 * if there is any entry, DNS response is sent out
 * if there is no entry then the original DNS request is forwarded

```
                            +--------------------+
                            |DNS lookup:         |
                            |   #LB Service names|
                            |   #EPG names       |
                            |   #Container names |
                            +--------------------+
                          DNS ^  |DNS       |DNS
                          Req |  |Resp      |Fwd (lookup failed)
                              |  v          v
                            +-+------------------+
+-----------+      DNS Req  |                    |       DNS Fwd   +-------------+
|           |-------------->|     Netplugin      +---------------->|  External   |
|Container#1|               |     datapath       |                 |  DNS        |
|           |<--------------|                    |<----------------+             |
+-----------+ DNS Resp      +--------------------+       DNS Resp  +-------------+
```

