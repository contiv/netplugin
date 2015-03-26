### Kubernetes Integration

[Kubernetes](https://github.com/GoogleCloudPlatform/kubernetes) infrastructure model is to create an infrastructure container (called pod).
This requires network plugin to create the network plumbing inside an infrastructure container, which is created dynamically.
And the visible names to the application is identified by pod-name or container-name(s) in the pod.

This network plugin has been enhanced to allow specification of the network container to be different from the application-container.
Further, kubernetes require that a plugin be written and kept in a specific directory which gets called when an applicaiton (aka pod) is launched.
This allows for a binary executable to be called with a specific parameters to do the network plumbing outside Kubernetes. 

For that reason, netplugin produces a new binary, called k8contivnet, a small plugin interface that will get called by Kubernetes
upon init of the plugin, and during creation/deletion of the application pod. The syntax of k8contivnet is as follows, which adheres to 
Kubernetes plugin requirements:

```
$ k8contivnet init
$ k8contivnet setup <pod-name> <pod-namespace> <infra-container-uuid>
$ k8contivnet teardown <pod-name> <pod-namespace> <infra-container-uuid>
$ k8contivnet help
```

This plugin would need to be copied in following directory:
`/usr/libexec/kubernetes/kubelet-plugins/net/exec/k8contivnet/k8contivnet`

In addition, kublet must be started with the plugin name 'k8contivnet' in order for this integration to work. For Kubernetes cluster setup, please refer to [Kubernetes Getting-started Guides](https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/getting-started-guides/ubuntu_multinodes_cluster.md)
