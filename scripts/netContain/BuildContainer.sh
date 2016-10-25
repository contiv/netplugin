#!/usr/bin/bash

function dockerBuildIt {
    imgId=`docker build $1 | grep "Successfully built" | cut -d " " -f 3`

    if [[ $imgId =~ [0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f] ]]; then
       echo "$2 Image has been built with ID $imgId"
       docker tag $imgId $2
       return 0
    fi
    echo "$2 Image was not built properly"
    return 255
}

set -x 

#eexists=`docker images contivbase | grep -w "contivbase" | wc -l`
#if [ $eexists != 0 ]; then
#    echo "An image by name contivbase already exists"
#    echo "Remove contivbase (docker rmi contivbase) and retry"
#    exit
#fi

ARG1=${1:-none}
if [ $ARG1 == "reinit" ]; then
   etcdctl rm -recursive /contiv.io
   etcdctl rm -recursive /docker/network
fi

sudo modprobe openvswitch

imgName="Contiv"
dockerBuildIt . $imgName
if [ $? != 0 ]; then
   echo "Failed building Contiv Image Bailing out Err $?"
   exit
fi

docker run --name=$imgName $imgName  2> /dev/null

echo "Copying the Contiv Binaries from the built container"
docker cp $imgName:/go/bin/netplugin scripts/netContain/
docker cp $imgName:/go/bin/netmaster scripts/netContain/
docker cp $imgName:/go/bin/netctl scripts/netContain/
docker cp $imgName:/go/bin/contivk8s scripts/netContain/


echo "Removing Intermediate Contiv Container"
docker rm -f $imgName
docker rmi -f $imgName


dockerBuildIt scripts/netContain contivbase
if [ $? != 0 ]; then
   echo "Failed building Contiv OVS Container Image, Bailing out Err $?"
   exit
fi

#echo "Build Contiv Image $imgId ..., tagging it as contivbase"
# docker tag contivbase netplugin
