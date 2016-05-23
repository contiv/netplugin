#!/usr/bin/bash

function dockerBuildIt {
    imgId=`docker build $1 | grep "Successfully built" | cut -d " " -f 3`

    if [[ $imgId =~ [0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f] ]]; then
       echo "$2 Image has been built with ID $imgId"
       return 0
    fi
    echo "$2 Image was not built properly"
    return 255
}

set -x 

eexists=`docker images contivbase | grep -w "contivbase" | wc -l`
if [ $eexists != 0 ]; then
    echo "An image by name contivbase already exists"
    echo "Remove contivbase (docker rmi contivbase) and retry"
    exit
fi

ARG1=${1:-none}
if [ $ARG1 == "reinit" ]; then
   etcdctl rm -recursive /contiv.io
   etcdctl rm -recursive /docker/network
fi

sudo modprobe openvswitch

imgId="Contiv"
dockerBuildIt . $imgId
if [ $? != 0 ]; then
   echo "Failed building Contiv Image Bailing out Err $?"
   exit
fi

docker run --name=$imgId $imgId  2> /dev/null

echo "Copying the Contiv Binaries from the built container"
docker cp $imgId:/go/bin/netplugin scripts/netContain/
docker cp $imgId:/go/bin/netmaster scripts/netContain/
docker cp $imgId:/go/bin/netctl scripts/netContain/


echo "Removing Intermediate Contiv Container"
docker rm -f $imgId
docker rmi -f $imgId


dockerBuildIt scripts/netContain contivbase
if [ $? != 0 ]; then
   echo "Failed building Contiv OVS Container Image, Bailing out Err $?"
   exit
fi

docker tag $imgId contivbase


scripts/netContain/contivd.sh start $ARG1
