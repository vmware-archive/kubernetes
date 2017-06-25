#!/bin/bash
kubectl create -f node01-deployment.yaml
kubectl create -f node02-deployment.yaml
kubectl create -f node03-deployment.yaml
kubectl create -f node04-deployment.yaml
