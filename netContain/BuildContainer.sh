#!/usr/bin/bash

function dockerBuildIt {
    imgId=`docker build $1 | grep "Successfully built" | cut -d " " -f 3`


    if [[ $imgId =~ [0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f][0-9a-f] ]]; then
       echo "$2 Image has been built with ID $imgId"
       return $imgId
    fi
    echo "$2 Image was not built properly"
    exit
}

set -x 

dockerBuildIt . Contiv
imgId=$?
docker run --name=$imgId $imgId 

echo "Copying the Contiv Binaries from the built container"
docker cp $imgId:/go/bin/netplugin netContain/
docker cp $imgId:/go/bin/netmaster netContain/
docker cp $imgId:/go/bin/netctl netContain/


echo "Removing Intermediate Contiv Container"
docker rm -f $imgId
docker rmi -f $imgId


dockerBuildIt netContain contivBase
imgId=$?
docker tag imgId contivBase

sudo mkdir -p /var/log/openvswitch

docker run -itd --name=contiv2  --privileged   -v /etc/openvswitch:/etc/openvswitch -v /var/run/:/var/run -v /var/log/openvswitch:/var/log/openvswitch contiv2 bash

