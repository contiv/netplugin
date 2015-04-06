#!/bin/bash
for i in `seq 9 20`;
do
    sudo /bin/mknod -m640 /dev/loop$i b 7 $i
    sudo /bin/chown root:disk /dev/loop$i
done
