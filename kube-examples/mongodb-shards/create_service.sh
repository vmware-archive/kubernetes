#!/bin/bash
kubectl create -f node01-service.yaml
kubectl create -f node02-service.yaml
kubectl create -f node03-service.yaml
kubectl create -f node04-service.yaml
