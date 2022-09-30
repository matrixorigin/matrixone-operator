#!/bin/sh

set -eu

nohup dockerd-entrypoint.sh &

sleep 10

kind create cluster --config=config.yml --name playground
export KUBECONFIG=/root/.kube/config
kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=60s

kubectl create ns mo
kubectl create ns mo-system

helm install mo mo/matrixone-operator --set image.tag=sha-c1a16ce --version 0.1.0 -n mo-system
kubectl wait --for=condition=Ready pods --all -n mo-system --timeout=300s

echo "> Wait for webhook certificate inject"
sleep 30

kubectl apply -f cluster.yml -n mo

tail -f /dev/null
