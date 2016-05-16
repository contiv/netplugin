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

modprobe openvswitch

imgId="Contiv"
dockerBuildIt . $imgId
if [ $? != 0 ]; then
   echo "Failed building Contiv Image Bailing out Err $?"
   exit
fi
docker run --name=$imgId $imgId 

echo "Copying the Contiv Binaries from the built container"
docker cp $imgId:/go/bin/netplugin netContain/
docker cp $imgId:/go/bin/netmaster netContain/
docker cp $imgId:/go/bin/netctl netContain/


echo "Removing Intermediate Contiv Container"
docker rm -f $imgId
docker rmi -f $imgId


dockerBuildIt netContain contivbase
if [ $? != 0 ]; then
   echo "Failed building Contiv OVS Container Image, Bailing out Err $?"
   exit
fi

docker tag $imgId contivbase

sudo mkdir -p /var/log/contiv
sudo mkdir -p /var/run/openvswitch

docker run -itd --net=host --name=contivNet  --privileged   -v /etc/openvswitch:/etc/openvswitch -v /var/run/:/var/run -v /var/log/contiv:/var/log/contiv contivbase bash
