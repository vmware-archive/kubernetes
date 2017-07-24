#!/bin/bash
#### ---- Install Package Dependencies ---- ####
apt-get update && apt-get install -y curl git jq
rm -rf /var/lib/apt/lists/*

export DEBIAN_FRONTEND="noninteractive"
export GOVERSION="1.8.3"
export GOROOT="/opt/go"
export GOPATH="/root/.go"
mkdir $GOPATH

cd /opt
curl -LO https://storage.googleapis.com/golang/go${GOVERSION}.linux-amd64.tar.gz
tar zxf go${GOVERSION}.linux-amd64.tar.gz && rm go${GOVERSION}.linux-amd64.tar.gz
ln -s /opt/go/bin/go /usr/bin/


curl -L https://github.com/vmware/govmomi/releases/download/v0.15.0/govc_linux_amd64.gz | gunzip > /usr/local/bin/govc
chmod +x /usr/local/bin/govc
govc version

curl -L https://github.com/y13i/j2y/releases/download/v0.0.8/j2y-linux_amd64.zip | gunzip > /usr/bin/j2y
chmod +x /usr/bin/j2y
j2y --version

cd /opt
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl

