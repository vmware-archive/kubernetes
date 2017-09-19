#!/bin/bash
# Generate Private and Public key in the Container
rm -rf /root/.ssh
echo -e "\n" | ssh-keygen -N "" &> /dev/null

# Get Node Addresses
addresses=`kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}'`
IFS=' ' read -a addressArray <<< "${addresses}"

for address in "${addressArray[@]}"
do
	./copy_pub_key.exp ${address} 'root' 'kubernetes'
done
