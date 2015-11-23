## dockcontivnet - netplugin plugin for docker

### To bootstrap: 

```
curl https://experimental.docker.com | sudo bash
mkdir -p /run/docker/plugins /etc/docker/plugins
echo 'http://127.0.0.1:4545' /etc/docker/plugins/netplugin.spec
```

And restart docker.

### Test:

```
docker network create -d netplugin test
docker run -it --publish-service foo.test ubuntu
```

You should see netplugin enter an ovs port into the namespace.
