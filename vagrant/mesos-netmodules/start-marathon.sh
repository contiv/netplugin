#!/bin/bash

docker run -d --name marathon -e MARATHON_MASTER=zk://localhost:2181/mesos \
-e MARATHON_ZK=zk://localhost:2181/marathon --net host -e MARATHON_HOSTNAME=10.0.2.15 \
-e MARATHON_MESOS_ROLE=public mesosphere/marathon:v1.1.1 --default_accepted_resource_roles "*"
