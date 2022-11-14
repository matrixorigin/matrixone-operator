#!/bin/sh

set -eu

nohup dockerd-entrypoint.sh &

sleep 10

# create kind cluster 1 control plane, 3 worker nodes
kind create cluster --config=./playground/config.yml --name playground
export KUBECONFIG=/root/.kube/config
kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=60s

# create namespace for operator and mo cluster
kubectl create ns mo-system
kubectl create ns matrixone

# install charts of matrixone-operator
helm repo add openkruise  https://openkruise.github.io/charts/

helm dependency build ./charts/matrixone-operator

helm install mo ./charts/matrixone-operator -n mo-system
kubectl wait --for=condition=Ready pods --all -n mo-system --timeout=300s

echo "> Wait for webhook certificate inject"
sleep 30

# deploy a matrixone cluster
kubectl apply -f ./playground/mo-playground.yaml -n matrixone

echo "> Wait MatrixOne Ready"
kubectl wait --for=condition=Ready mo --all -n matrixone --timeout=600s

echo "> Welcome to MatrixOne"

kubectl port-forward svc/mo-tp-cn 6001:6001 -n matrixone --address='0.0.0.0'
