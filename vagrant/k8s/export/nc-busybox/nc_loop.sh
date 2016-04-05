#!/bin/sh

while true; do
/usr/bin/nc -ltz -p 6379
sleep 5
done

