**Kubernetes network policy in Contiv**

kubernetes network policy can be used in Contiv.
Each k8s network policy will be translated into multiple Contiv policies.
The following Contiv labels can be used to configure network policy,
```
    io.contiv.tenant: <tenant-name>
    io.contiv.network: <network-name>
    io.contiv.net-group: <endpoint-group-name>
```

**how to configure:**

1. configure tenant and network.
   tenant 'default' is created by default.
   
   ```netctl net create -s 20.20.20.0/24 -g 20.20.20.1 default-net ``` 

2. create network policy yaml and apply

    configure podSelector->matchLabels with Contiv labels.
```
    io.contiv.tenant: <tenant-name>
    io.contiv.network: <network-name>
    io.contiv.net-group: <endpoint-group-name>
```
   configure ingress pod selector with Contiv labels
    
```
    io.contiv.tenant: <tenant-name>
    io.contiv.network: <network-name>
    io.contiv.net-group: <endpoint-group-name>
```
   configure ingress rules with protocol & port

3. create pods with Contiv labels,
```
    io.contiv.tenant: <tenant-name>
    io.contiv.network: <network-name>
    io.contiv.net-group: <endpoint-group-name>
```
```
   kubectl run db1 --image=alpine -l io.contiv.network=default-net,io.contiv.net-group=db sleep 5000 
   kubectl run frontend1 --image=alpine -l io.contiv.network=default-net,io.contiv.net-group=frontend sleep 5000 
```

example:
create a network policy to allow port 80/5000 from 'frontend' pods to 'db' pods

```
cat > test-policy.yaml <<EOL
apiVersion: extensions/v1beta1
kind: NetworkPolicy
metadata:
 name: test-policy
 namespace: default
spec:
 podSelector:
  matchLabels:
    io.contiv.network: default-net
    io.contiv.net-group: db
 ingress:
  - from:
     - namespaceSelector:
        matchLabels:
         project: myproject
     - podSelector:
        matchLabels:
         io.contiv.network: default-net
         io.contiv.net-group: frontend
    ports:
     - protocol: tcp
       port: 5000
     - protocol: udp
       port: 5000
     - protocol: tcp
       port: 80
     - protocol: udp
       port: 80
EOL
```

apply policy 
```
kubectel create -f test-policy.yaml
```
network policy will be translated to epg/policy as below,
```
$ netctl group ls
Tenant   Group     Network      IP Pool   Policies  Network profile
------   -----     -------      --------  ---------------
default  db        default-net            db-policy        
default  frontend  default-net            frontend-policy  
```
```
$ netctl policy ls
Tenant   Policy
------   ------
default  db-policy
default  frontend-policy
```
```
$ netctl policy rule-ls db-policy
Incoming Rules:
Rule                  Priority  From EndpointGroup  From Network  From IpAddress  Protocol  Port  Action
----                  --------  ------------------  ------------  ---------       --------  ----  ------
deny-all-0-in         1                                                                     0     deny
frontend-udp-5000-in  2         frontend                                          udp       5000  allow
frontend-tcp-5000-in  2         frontend                                          tcp       5000  allow
frontend-tcp-80-in    2         frontend                                          tcp       80    allow
frontend-udp-80-in    2         frontend                                          udp       80    allow
Outgoing Rules:
Rule                   Priority  To EndpointGroup  To Network  To IpAddress  Protocol  Port  Action
----                   --------  ----------------  ----------  ---------     --------  ----  ------
deny-all-0-out         1                                                               0     deny
frontend-udp-5000-out  2         frontend                                    udp       5000  allow
frontend-tcp-80-out    2         frontend                                    tcp       80    allow
frontend-tcp-5000-out  2         frontend                                    tcp       5000  allow
frontend-udp-80-out    2         frontend                                    udp       80    allow
```

```
$ netctl policy rule-ls frontend-policy
Incoming Rules:
Rule           Priority  From EndpointGroup  From Network  From IpAddress  Protocol  Port  Action
----           --------  ------------------  ------------  ---------       --------  ----  ------
deny-all-0-in  1                                                                     0     deny
Outgoing Rules:
Rule            Priority  To EndpointGroup  To Network  To IpAddress  Protocol  Port  Action
----            --------  ----------------  ----------  ---------     --------  ----  ------
deny-all-0-out  1                                                               0     deny
```