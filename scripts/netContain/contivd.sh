#!/usr/bin/bash

set -x

function usage() {
	echo "contivd.sh  { {start | restart} [reinit] } | {stop}"
}

sudo mkdir -p /var/log/contiv
sudo mkdir -p /var/run/openvswitch

eexists=$(docker images contivbase | grep -w "contivbase" | wc -l)
if [ $eexists == 0 ]; then
	echo "contivbase image has not been created, First Build contivbase"
	exit
fi

arg=${1:-none}

for arg in $*; do
	case $arg in
		"reinit")
			reinitContiv=true
			;;
		"restart")
			restartContiv=true
			;;
		"stop")
			docker stop contivNet
			exit
			;;
		"start")
			startNow=true
			;;
		"none")
			usage
			exit
			;;
	esac
done

spawned=$(docker ps | grep -w "contivNet" | wc -l)
stopped=$(docker ps -a | grep -w "contivNet" | grep "Exited" | wc -l)

if [ $startNow ]; then
	if [ $spawned != 0 ]; then
		echo "contivNet is Already Running, Try Stopping or restart"
		exit
	fi
fi

if [ $restartContiv ]; then
	if [ $spawned == 0 ] && [ $stopped == 0 ]; then
		echo "contivNet has not been spawned, Try Start"
		exit
	fi
	if [ $stopped == 0 ]; then
		docker stop contivNet
		stopped=1
	fi
fi

reinitArg=""
if [ $reinitContiv ]; then
	docker rm -f contivNet
	reinitArg="reinit"
	etcdctl rm -recursive /contiv.io
	etcdctl rm -recursive /docker/network
fi

sudo modprobe openvswitch
if [ $stopped != 0 ]; then
	docker start contivNet
else
	docker run -itd --net=host --name=contivNet --privileged -v /etc/openvswitch:/etc/openvswitch -v /var/run/:/var/run -v /var/log/contiv:/var/log/contiv contivbase "$reinitArg"
fi
