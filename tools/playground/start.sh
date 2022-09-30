#!/bin/sh
set -eu

nohup dockerd-entrypoint.sh &

sleep 10

kind create cluster --config=config.yml --name playground
kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=60s
kubectl create ns matrixone
kubectl create ns matrixone-operator
helm install op mo/matrixone-operator --version 0.1.0 -n matrixone-operator
kubectl wait --for=condition=Ready pods --all -n matrixone-operator --timeout=60s
kubectl apply -f cluster.yml -n matrixone

tail -f /dev/null
