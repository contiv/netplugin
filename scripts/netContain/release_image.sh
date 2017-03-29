#!/bin/bash

contiv_version=""
docker_user="contiv"
docker_password=""
image_name="contiv/netplugin"
image_tag=""

function usage() {
	echo "Usage:"
	echo "./release_image.sh -v <contiv version> -u <docker user> -p <docker password> -i <image name> -t <image tag>"
	echo "Example: ./release_image.sh -v v0.1-11-30-2016.20-08-20.UTC -u contiv -i contiv/netplugin"
	echo "Released versions are available from https://github.com/contiv/netplugin/releases"
	echo "Default values are:"
	echo "User:contiv, image contiv/netplugin and tag contiv version"
	echo "Omit -p to provide password interactively"
	exit 1
}

function error_ret() {
	echo ""
	echo $1
	exit 1
}

while getopts ":v:u:p:i:t:" opt; do
	case $opt in
		v)
			contiv_version=$OPTARG
			;;
		u)
			docker_user=$OPTARG
			;;
		p)
			docker_password=$OPTARG
			;;
		i)
			image_name=$OPTARG
			;;
		t)
			image_tag=$OPTARG
			;;
		:)
			echo "An argument required for $OPTARG was not passed"
			usage
			;;
		?)
			usage
			;;
	esac
done

if [ "$contiv_version" = "" ]; then
	usage
fi

if [ "$image_tag" = "" ]; then
	image_tag=$contiv_version
fi

cd -P -- "$(dirname -- "$0")"

echo "Login to docker hub as $docker_user user"
if [ "$docker_password" = "" ]; then
	docker login -u $docker_user
else
	docker login -u $docker_user -p $docker_password
fi

mkdir bin || true
wget https://github.com/contiv/netplugin/releases/download/$contiv_version/netplugin-$contiv_version.tar.bz2
tar xvfj netplugin-$contiv_version.tar.bz2 -C bin
# remove the contrib directory, we don't need it in the image
rm -rf bin/contrib || true

if [ "$?" != "0" ]; then
	error_ret "FAILED: Error getting contiv version $contiv_version"
fi

docker build . -t $image_name:$image_tag -t ${image_name}:latest
if [ "$?" != "0" ]; then
	error_ret "FAILED: Error building image for contiv version $contiv_version to $image_name:$image_tag"
fi

docker push $image_name:$image_tag
if [ "$?" = "0" ]; then
	echo ""
	echo "SUCCESS: Pushed contiv version $contiv_version to $image_name:$image_tag"
else
	error_ret "FAILED: Error pushing contiv version $contiv_version to $image_name:$image_tag"
fi
