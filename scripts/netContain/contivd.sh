#!/usr/bin/bash

set -x 

function usage {
	echo "contivd.sh  { {start | restart} [reinit] } | {stop}"
}

eexists=`docker images contivbase | grep -w "contivbase" | wc -l`
if [ $eexists == 0 ]; then
    echo "contivbase image has not been created, First Build contivbase"
    exit
fi

arg=${1:-none}

for arg in $*; do
    case $arg in
       "reinit" )
          reinitContiv=true
          ;;
       "restart" )
          restartContiv=true
          ;;
       "stop" )
          docker stop contivNet
          exit
          ;;
        "start" )
          startNow=true
          ;;
        "none" )
          usage
          exit
          ;;
    esac
done

eexists=`docker ps | grep -w "contivNet" | wc -l`

if [ $startNow ]; then
    if [ $eexists != 0]; then
        echo "contivNet is Already Running, Try Stopping or restart"
        exit
    fi
fi


if [ $restartContiv ]; then
    if [ $eexists == 0 ]; then
       echo "contivNet has not been spawned, Try Start"
       exit
    fi
    docker stop contivNet
fi


reinitArg=""
if [  $reinitContiv ]; then
    docker rm -f contivNet
    reinitArg="reinit"
    etcdctl rm -recursive /contiv.io
    etcdctl rm -recursive /docker/network
fi   

docker run -itd --net=host --name=contivNet  --privileged   -v /etc/openvswitch:/etc/openvswitch -v /var/run/:/var/run -v /var/log/contiv:/var/log/contiv contivbase "$reinitArg"
