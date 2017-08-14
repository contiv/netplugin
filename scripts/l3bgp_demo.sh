
#  "make l3bgp-demo" will bringup three netplugin VMs and two quagga VMs.
#  The quagga VMs are preconfigured with IP interfaces and BGP peer based
#  on the topology below. Netplugin needs to be configured with forwarding
#  mode as 'routing' and BGP peers based on the topology below.
#
topology=" 
      +-------------+       +------------+               +------------+      
      | netplugin-  |       | netplugin- |               | netplugin- |      
      |    node2    |       |    node3   |               |    node1   |      
      +------+------+       +----+-------+               +----+-------+      
    50.1.1.2 |                   | 60.1.1.1                   |80.1.1.2      
     AS65002 |                   |  AS65002                   |AS65002       
             |                   |                            |              
             |  +---------+      |             +---------+    |              
             |  |         |      |             |         |    |              
             +--+ quagga1 +------+             | quagga2 +----+              
      50.1.1.200|         |60.1.1.200          |         | 80.1.1.200        
         AS500  +-----+---+ AS500              +-----+---+   AS500           
                      |                              |                       
             70.1.1.1 +------------------------------+ 70.1.1.2              
"
#   Debug Notes:
#   When BGP is configured, netplugin creates an inband interface (inb01)
#   with the IP address specified in "--router-ip" option. Currently, it 
#   is expected to have an L2 adjacency between netplugin and BGP peer or
#   neighbor (specified in '--neighbor' option)
#
#                  +-----------------------+
#                  |                       |
#                  |    netplugin-node1    |
#                  |        (gobgp)        |
#                  |           +           |
#                  |           | inb01     |
#                  |  +--------+--------+  |
#                  |  |       OVS       |  |
#                  |  +--------+--------+  |
#                  +-----------+-----------+
#                              | uplink     
#                              |
#                              |
#                              +
#                          BGP Router
#

echo "$topology"
set -x
netctl global set --fwd-mode routing
sleep 3
netctl bgp create netplugin-node1 -router-ip="50.1.1.2/24" --as="65002" --neighbor-as="500" --neighbor="50.1.1.200"
netctl bgp create netplugin-node2 -router-ip="60.1.1.1/24" --as="65002" --neighbor-as="500" --neighbor="60.1.1.200"
netctl bgp create netplugin-node3 -router-ip="80.1.1.2/24" --as="65002" --neighbor-as="500" --neighbor="80.1.1.200"
