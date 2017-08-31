#!/bin/bash
#### ---- Install Package Dependencies ---- ####

pip install crudini
crudini --version

go version
go get -u github.com/vmware/govmomi/govc

cd /root
wget  https://github.com/y13i/j2y/releases/download/v0.0.8/j2y-linux_amd64.zip
unzip  j2y-linux_amd64.zip
chmod +x ./j2y
mv ./j2y /usr/local/bin/j2y
j2y --version

wget https://storage.googleapis.com/kubernetes-release/release/v1.7.4/bin/linux/amd64/kubectl
chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl
