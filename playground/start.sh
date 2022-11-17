#!/bin/sh

set -eu

#nohup dockerd-entrypoint.sh &

#sleep 10

# Create kind cluster 1 control plane, 3 worker nodes
kind create cluster --config=./playground/config.yml --name playground
#export KUBECONFIG=/root/.kube/config
kubectl wait --for=condition=Ready pods --all --all-namespaces --timeout=60s

# Create namespace for operator and mo cluster
kubectl create ns mo-system
kubectl create ns matrixone

# Install MinIO
kubectl apply -f playground/minio.yaml -n matrixone

echo "> Wait MinIO Ready"
kubectl wait --for=condition=Ready pods --all -n matrixone --timeout=300s

echo "> Create MatrixOne bucket"
kubectl apply -f playground/create-bucket-job.yaml -n matrixone

# Install MatrixOne Operator
helm dependency build ./charts/matrixone-operator

helm install mo ./charts/matrixone-operator -n mo-system
kubectl wait --for=condition=Ready pods --all -n mo-system --timeout=300s

echo "> Wait for webhook certificate inject"
sleep 30

echo "> Create MinIO secret"
kubectl  create secret generic minio --from-literal=AWS_ACCESS_KEY_ID=minio --from-literal=AWS_SECRET_ACCESS_KEY=minio123 -n matrixone

# Deploy a MatrixOne cluster
kubectl apply -f ./playground/mo-playground.yaml -n matrixone

echo "> Wait MatrixOne Ready"
kubectl wait --for=condition=Ready mo --all -n matrixone --timeout=600s

echo "> Welcome to MatrixOne"

kubectl port-forward svc/mo-tp-cn 6001:6001 -n matrixone --address='0.0.0.0'
