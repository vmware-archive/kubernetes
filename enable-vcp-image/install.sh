#!/bin/bash
#### ---- Install Package Dependencies ---- ####
apt-get update && apt-get install -y curl git jq
rm -rf /var/lib/apt/lists/*

export DEBIAN_FRONTEND="noninteractive"
export GOVERSION="1.8.3"
export GOROOT="/opt/go"
export GOPATH="/root/.go"

cd /opt
curl -LO https://storage.googleapis.com/golang/go${GOVERSION}.linux-amd64.tar.gz
tar zxf go${GOVERSION}.linux-amd64.tar.gz && rm go${GOVERSION}.linux-amd64.tar.gz
ln -s /opt/go/bin/go /usr/bin/

mkdir $GOPATH
go get github.com/vmware/govmomi/govc
ln -s /root/.go/bin/govc /usr/bin
govc version


go get github.com/y13i/j2y
ln -s /root/.go/bin/j2y /usr/bin
j2y --version

cd /opt
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl

