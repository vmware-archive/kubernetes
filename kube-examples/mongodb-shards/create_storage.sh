#!/bin/bash
kubectl create -f storageclass.yaml
kubectl create -f storage-volumes-node01.yaml
kubectl create -f storage-volumes-node02.yaml
kubectl create -f storage-volumes-node03.yaml
kubectl create -f storage-volumes-node04.yaml
