sudo make tar
cp netplugin-1.2.0-unsupported.tar.bz2 scripts/netContain/netplugin-1.2.1.tar.bz2

sudo ./scripts/netContain/release_image.sh -v 1.2.1
sudo docker tag contiv/netplugin:1.2.1 newton001/netplugin:1.2.2
sudo docker push newton001/netplugin:1.2.2
