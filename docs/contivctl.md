## contivctl

contivctl command line tool lets you add/modify/delete objects into contiv object model using REST api.

*Note:*
`contivctl` Command can be found under `$GOPATH/bin` directory (`/opt/gopath/bin` in vagrant VMs) after a build.
Currently, `contivctl` needs to be run on the same host where `netmaster` is running

## Usage

```
vagrant@netplugin-node1:~$ contivctl --help
usage: contivctl [-h] [--version] {policy,rule,group,network} ...

positional arguments:
  {policy,rule,group,network}
    policy              Policy operations
    rule                Rule add/delete
    group               Endpoint group operations
    network             Network operations

optional arguments:
  -h, --help            show this help message and exit
  --version             show program's version number and exit
```

###  Networks

```
vagrant@netplugin-node1:~$ contivctl network create --help
usage: contivctl network create [-h] [-tenantName TENANTNAME]
                                [-public {yes,no}] [-encap {vlan,vxlan}]
                                -subnet SUBNET -gateway DEFAULTGW
                                networkName

positional arguments:
  networkName           Network name

optional arguments:
  -h, --help            show this help message and exit
  -tenantName TENANTNAME
  -public {yes,no}      Is this a public network
  -encap {vlan,vxlan}   Packet tag
  -subnet SUBNET        Subnet addr/mask
  -gateway DEFAULTGW  default GW
```

#### Examples

Following creates a vxlan network called `production` under `default` tenant and assigns a subnet to it

```
vagrant@netplugin-node1:~$ contivctl network create production -encap=vxlan -subnet=20.20.20.0/24 -gateway=20.20.20.254
Creating network default:production
Network Create response is: {"key":"default:production","gateway":"20.20.20.254","encap":"vxlan","isPrivate":true,"networkName":"production","subnet":"20.20.20.0/24","tenantName":"default","link-sets":{},"links":{"Tenant":{"type":"tenant","key":"default"}}}
```

Following lists all networks under `default` tenant

```
vagrant@netplugin-node1:~$ contivctl network list
Listing all networks for tenant default
Network		Public	Encap	Subnet			Gateway
private		No	vxlan	10.1.0.0/16		10.1.254.254
public		Yes	vlan	192.168.1.0/24		192.168.1.254
production		No	vxlan	20.20.20.0/24		20.20.20.254
```

Following deletes a network from `default` tenant

```
vagrant@netplugin-node1:~$ contivctl network delete production
Deleting network default:production
```

### Policies

#### Usage

```
vagrant@netplugin-node1:~$ contivctl policy create --help
usage: contivctl policy create [-h] [-tenantName TENANTNAME] policyName

positional arguments:
  policyName            Policy name

optional arguments:
  -h, --help            show this help message and exit
  -tenantName TENANTNAME

```

#### Example

Following creates a policy named `webTier`
```
vagrant@netplugin-node1:~$ contivctl policy create webTier
Creating policy default:webTier
Create policy response is: {"key":"default:webTier","policyName":"webTier","tenantName":"default","link-sets":{},"links":{"Tenant":{}}}
```

Following lists all policies under `default` tenant

```
vagrant@netplugin-node1:~$ contivctl policy list
Listing all policies for tenant default
Tenant,		Policy
-----------------------------------
default		first
default		webTier
```

### Rules

#### Usage
```
vagrant@netplugin-node1:~$ contivctl rule add --help
usage: contivctl rule add [-h] [-tenantName TENANTNAME]
                          [-direction {in,out,both}] [-priority PRIORITY]
                          [-endpointGroup ENDPOINTGROUP] [-network NETWORK]
                          [-ipAddress IPADDRESS]
                          [-protocol {tcp,udp,icmp,igmp}] [-port PORT]
                          [-action {accept,deny}]
                          policyName ruleId

positional arguments:
  policyName            Policy name
  ruleId                Rule identifier

optional arguments:
  -h, --help            show this help message and exit
  -tenantName TENANTNAME
  -direction {in,out,both}
  -priority PRIORITY    priority [1..100]
  -endpointGroup ENDPOINTGROUP
                        Name of endpoint group
  -network NETWORK      Name of network
  -ipAddress IPADDRESS  IP address/mask
  -protocol {tcp,udp,icmp,igmp}
                        IP protocol
  -port PORT            tcp/udp port number
  -action {accept,deny}
                        Accept or deny
```

All rules have a rule Id which uniquely identifies the rule under a policy. Rule Id is a string that can be anything user chooses. Rule id does not specify the order in which rules are applied. User has to specify the priority for the rule. Higher the priority, higher preference a rule gets. In following example, we add a default deny rule which uses default priority 1 and a allow web traffic rule which is at higher priority 10.

#### Examples

Following adds a default deny rule to the policy

```
vagrant@netplugin-node1:~$ contivctl rule add webTier 1 -direction=in -action=deny
Adding rule to pilicy rule default:webTier
rule create, sending: {"direction": "in", "protocol": "", "ruleId": "1", "port": 0, "policyName": "webTier", "network": null, "priority": 1, "tenantName": "default", "endpointGroup": null, "action": "deny", "ipAddress": null}
Rule add response is: {"key":"default:webTier:1","action":"deny","direction":"in","policyName":"webTier","priority":1,"ruleId":"1","tenantName":"default","link-sets":{"Policies":{"default:webTier":{"type":"policy","key":"default:webTier"}}}}
```

Following adds a rule to allow incoming traffic on port 80. Note that it has a higher priority 10.

```
vagrant@netplugin-node1:~$ contivctl rule add webTier 2 -direction=in -priority 10 -protocol=tcp -port=80 -action=accept
Adding rule to pilicy rule default:webTier
rule create, sending: {"direction": "in", "protocol": "tcp", "ruleId": "2", "port": 80, "policyName": "webTier", "network": null, "priority": 10, "tenantName": "default", "endpointGroup": null, "action": "accept", "ipAddress": null}
Rule add response is: {"key":"default:webTier:2","action":"accept","direction":"in","policyName":"webTier","port":80,"priority":10,"protocol":"tcp","ruleId":"2","tenantName":"default","link-sets":{"Policies":{"default:webTier":{"type":"policy","key":"default:webTier"}}}}
```

Following lists all the rules under `webTier` policy

```
vagrant@netplugin-node1:~$ contivctl rule list webTier
Listing all rules for policy default:webTier
Rule, direction, priority, endpointGroup, network, ipAddress, protocol, port, action
---------------------------------------------------------------------------------------------
1, in, 1, --, --, --, --, --, deny
2, in, 10, --, --, --, tcp, 80, accept
```

Following deletes a rule from `webTier` policy

```
vagrant@netplugin-node1:~$ contivctl rule delete webTier 2
Deleting rule default:webTier:2
```

### Endpoint Groups

```
vagrant@netplugin-node1:~$ contivctl group create --help
usage: contivctl group create [-h] [-tenantName TENANTNAME]
                              [-networkName NETWORKNAME] [-policies POLICIES]
                              groupName

positional arguments:
  groupName             Endpoint group name

optional arguments:
  -h, --help            show this help message and exit
  -tenantName TENANTNAME
  -networkName NETWORKNAME
                        Network name
  -policies POLICIES    List of policies
```

#### Examples

Following creates an endpoint group called 'webTier.production' and associates 'webTier' policy to it

```
vagrant@netplugin-node1:~$ contivctl group create webTier.production -policies=webTier
Creating endpoint group default:webTier.production
Epg Create response is: {"key":"default:webTier.production","endpointGroupId":5,"groupName":"webTier.production","policies":["webTier"],"tenantName":"default","link-sets":{"Policies":{"default:webTier":{"type":"policy","key":"default:webTier"}}},"links":{"Network":{},"Tenant":{"type":"tenant","key":"default"}}}
```

Following lists all endpoint groups under default tenant

```
vagrant@netplugin-node1:~$ contivctl group list
Listing all endpoint groups for tenant default
Group		Network		Policies
---------------------------------------------------
webTier.production		--		webTier
srv0.private		--		first
srv1.private		--		first
srv2.private		--		first
srv3.private		--		first
```
