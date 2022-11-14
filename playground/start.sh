#!/bin/sh

set -eu

nohup dockerd-entrypoint.sh &

sleep 10

kind create cluster --config=./playground/config.yml --name playground
export KUBECONFIG=/root/.kube/config
kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=60s

kubectl create ns mo-system

helm repo add openkruise  https://openkruise.github.io/charts/

helm dependency build ./charts/matrixone-operator

helm install mo ./charts/matrixone-operator -n mo-system
kubectl wait --for=condition=Ready pods --all -n mo-system --timeout=300s

echo "> Wait for webhook certificate inject"
sleep 30

kubectl apply -f ./examples/mo-cluster-playground.yaml -n mo-system

echo "> Welcome to matrixone"

tail -f /dev/null
