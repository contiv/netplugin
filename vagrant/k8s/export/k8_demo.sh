printf "\n********************************************************************\n"
printf "\nSetup demo networks"
printf "\n********************************************************************\n"
cp /shared/netctl /bin/
netctl net create -t default --subnet=20.1.1.0/24 default-net
netctl net create -t default --subnet=21.1.1.0/24 poc-net 
netctl group create -t default default-net  default-epg 
netctl group create -t default poc-net  poc-epg 

printf "\n********************************************************************\n"
printf "Example 1: No network labels = Pod placed in default network"
printf "\n********************************************************************\n"
cd /shared
kubectl create -f defaultnet-busybox1.yaml 
kubectl create -f defaultnet-busybox2.yaml 
printf "Waiting for the pods to startup\n"
until kubectl get pods defaultnet-busybox1 | grep Running
do
   sleep 1
done
until kubectl get pods defaultnet-busybox2 | grep Running
do
   sleep 1
done
kubectl exec defaultnet-busybox1 -- ip address
kubectl exec defaultnet-busybox1 -- ping -c 3 $(kubectl describe pod defaultnet-busybox2 | grep IP | awk '{print $2}')

printf "\n********************************************************************\n"
printf "Example 2: Use network labels to specify a network and epg for the Pod"
printf "\n********************************************************************\n"
kubectl create -f pocnet-busybox.yaml 
printf "Waiting for the pods to startup\n"
until kubectl get pods busybox-poc-net | grep Running
do
   sleep 1
done
kubectl exec busybox-poc-net -- ip address

printf "\n********************************************************************\n"
printf "Example 3: Use Contiv to specify and enforce network policy"
printf "\n********************************************************************\n"
./policy.sh
 netctl rule list icmpPol

kubectl create -f noping-busybox.yaml 
kubectl create -f pingme-busybox.yaml

printf "Waiting for the pods to startup\n"
until kubectl get pods annoyed-busybox | grep Running
do
   sleep 1
done
until kubectl get pods sportive-busybox | grep Running
do
   sleep 1
done
kubectl describe pod annoyed-busybox | grep IP
kubectl describe pod sportive-busybox | grep IP

printf "\nannoyed-busybox should not be accessible"
printf "\n....................................................................\n"
kubectl exec busybox-poc-net -- ping -c 3 $(kubectl describe pod annoyed-busybox | grep IP | awk '{print $2}')
printf "\nsportive-busybox should be accessible"
printf "\n....................................................................\n"
kubectl exec busybox-poc-net -- ping -c 3 $(kubectl describe pod sportive-busybox | grep IP| awk '{print $2}')


