In this example we will simulate a two host cluster (with the two hosts being simulated by two vagrant vms) and deploy a multi vlan configuration on them. The logical topology looks similar to one shown below:

![VlanNetwork](./VlanNetwork.jpg)

1. Launch the two node setup:

    ```
    cd $GOPATH/src/github.com/contiv/netplugin
    CONTINV_NODES=2 make demo
    ```
    
    Note: User may simulate more hosts by setting the CONTINV_NODES variable to a desired number. Each host corresponds to a vagrant-vm, which are connected through their 'eth2' interface to a virtualbox bridge network for the container data traffic.
    
2. Once the make is done, start a separate ssh session to each node and run the netplugin:

    ```
    CONTINV_NODES=2 vagrant ssh netplugin-node1
    sudo $GOPATH/bin/netplugin -host-label host1
    ```
    ```
    CONTINV_NODES=2 vagrant ssh netplugin-node2
    sudo $GOPATH/bin/netplugin -host-label host2
    ```
3. Let's create 4 containers two on each vagrant node that we will add to the two networks viz. orange and purple later.

    On netplugin-node1:
    ```
    sudo docker run -it --name=myContainer1 --hostname=myContainer1 ubuntu /bin/bash
    sudo docker run -it --name=myContainer2 --hostname=myContainer2 ubuntu /bin/bash
    ```
    On netplugin-node2:
    ```
    sudo docker run -it --name=myContainer3 --hostname=myContainer1 ubuntu /bin/bash
    sudo docker run -it --name=myContainer4 --hostname=myContainer2 ubuntu /bin/bash
    ```
4. Now let's load the multi-host multi vlan configuration from the [../examples/two_hosts_multiple_vlans_nets.json](../examples/two_hosts_multiple_vlans_nets.json) file by issuing the following commands from one of the vagrant vms.

    ```
    CONTINV_NODES=2 vagrant ssh netplugin-node1
    cd $GOPATH/src/github.com/contiv/netplugin
    netdcli -cfg examples/two_hosts_multiple_vlans_nets.json
    ```
5. Now everything should be setup as per the diagram and we are good to test the connectivity.

    Determine the IP addresses assigned to the container `myContainer3` and `myContainer4` by running command like `ifconfig` or `ip address show` from the container shells (opened in step 3).
    Note: the current implementation with ovs names the netdevices as 'port<number>'

    ```
root@myContainer3:/# ip address show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host
       valid_lft forever preferred_lft forever
7: eth0: <BROADCAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 02:42:ac:11:00:02 brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.2/16 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::42:acff:fe11:2/64 scope link
       valid_lft forever preferred_lft forever
11: port2: <BROADCAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UNKNOWN group default
    link/ether 16:bd:b0:78:aa:26 brd ff:ff:ff:ff:ff:ff
    inet 11.1.2.2/24 scope global port2
       valid_lft forever preferred_lft forever
    inet6 fe80::14bd:b0ff:fe78:aa26/64 scope link
       valid_lft forever preferred_lft forever
root@myContainer3:/#
    ```

    Go to the terminal for the container `myContainer1` and ping the ip for the container `myContainer3`. The ping succeeds as the containers belong to same vlan network.

    ```
root@myContainer1:/# ping -c3 11.1.2.2
PING 11.1.2.2 (11.1.2.2) 56(84) bytes of data.
64 bytes from 11.1.2.2: icmp_seq=1 ttl=64 time=3.15 ms
64 bytes from 11.1.2.2: icmp_seq=2 ttl=64 time=1.36 ms
64 bytes from 11.1.2.2: icmp_seq=3 ttl=64 time=9.54 ms

--- 11.1.2.2 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2003ms
rtt min/avg/max/mdev = 1.365/4.688/9.541/3.509 ms
root@myContainer1:/#
    ```

    Now from `myContainer1`, ping the ip for `myContainer4`. The ping fails as the containers belong to different vlan networks.

    ```
root@myContainer1:/# ping -c3 11.1.3.2
PING 11.1.3.2 (11.1.3.2) 56(84) bytes of data.

--- 11.1.3.2 ping statistics ---
3 packets transmitted, 0 received, 100% packet loss, time 2016ms

root@myContainer1:/#
    ```
