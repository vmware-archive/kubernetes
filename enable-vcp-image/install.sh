#!/bin/bash
#### ---- Install Package Dependencies ---- ####
pip install crudini
crudini --version

go version
go get -u github.com/vmware/govmomi/govc
pip install json2yaml
json2yaml --version
yaml2json --version

cd /root
wget https://storage.googleapis.com/kubernetes-release/release/v1.7.4/bin/linux/amd64/kubectl
chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl
