#!/usr/bin/env python3
# Copyright 2026 Matrix Origin
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
"""Fresh-flow CNI contract probe; requires an explicit kubeconfig and Python image.

Creates only a uniquely named namespace, deletes it on exit. This tests actual
CNI policy semantics, not the Operator reconciler or the CN Gossip application.
The corresponding Go tests exercise reconcile/selector/ownership decisions.
"""
import argparse
import json
import subprocess
import time
import uuid

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--kubeconfig', required=True)
parser.add_argument('--image', required=True, help='Image containing python; use a pinned digest')
args = parser.parse_args()
namespace = 'udf-net-' + uuid.uuid4().hex[:10]
base = ['kubectl', '--kubeconfig=' + args.kubeconfig, '--request-timeout=15s']


def kubectl(*command, data=None):
    result = subprocess.run(base + list(command), input=None if data is None else json.dumps(data),
                            text=True, capture_output=True, timeout=40, check=True)
    return result.stdout.strip()


def apply(obj):
    return kubectl('apply', '-f', '-', data=obj)


server = '''import socket,threading,time

def tcp(port,host):
 s=socket.socket();s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1);s.bind((host,port));s.listen()
 def serve():
  while True:
   c,_=s.accept();c.sendall(b"ok");c.close()
 threading.Thread(target=serve,daemon=True).start()
for port in (6001,6002,6003,6004,6005,6006,7001):tcp(port,"0.0.0.0")
tcp(50051,"127.0.0.1")
s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM);s.bind(("0.0.0.0",6005))
while True:
 data,address=s.recvfrom(100);s.sendto(data,address)
'''
probe = '''import socket,sys,json
ip=sys.argv[1];out=[]
for udp,port in ((False,6001),(True,6005),(False,50051)):
 s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM if udp else socket.SOCK_STREAM);s.settimeout(0.5)
 try:
  s.connect((ip,port))
  if udp:s.send(b"ok")
  out.append(s.recv(2)==b"ok")
 except (OSError,TimeoutError):out.append(False)
 finally:s.close()
print(json.dumps(out))
'''


def pod(name, trusted=False):
    return {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'name': name, 'namespace': namespace,
            'labels': {'app': 'cn' if name == 'cn' else 'client', 'trusted': str(trusted).lower()}},
            'spec': {'automountServiceAccountToken': False, 'containers': [{'name': 'python',
            'image': args.image, 'imagePullPolicy': 'IfNotPresent', 'command': ['python', '-u', '-c',
            server if name == 'cn' else 'import time;time.sleep(3600)'],
            'resources': {'requests': {'cpu': '10m', 'memory': '32Mi'},
                          'limits': {'cpu': '250m', 'memory': '128Mi'}}}]}}


def policy(name, ingress):
    return {'apiVersion': 'networking.k8s.io/v1', 'kind': 'NetworkPolicy',
            'metadata': {'name': name, 'namespace': namespace},
            'spec': {'podSelector': {'matchLabels': {'app': 'cn'}},
                     'policyTypes': ['Ingress'], 'ingress': ingress}}


def observe(client, address):
    return json.loads(kubectl('-n', namespace, 'exec', client, '--', 'python', '-c', probe, address))


def check(label, address, trusted, untrusted):
    # CNI policy distribution is asynchronous. Convergence has a fixed deadline;
    # each observation uses new sockets. Once converged verify three fresh flows.
    deadline = time.monotonic() + 20
    expected = [trusted, untrusted]
    while True:
        actual = [observe('trusted', address), observe('untrusted', address)]
        if actual == expected:
            break
        if time.monotonic() >= deadline:
            raise AssertionError((label, expected, actual))
    for _ in range(3):
        actual = [observe('trusted', address), observe('untrusted', address)]
        assert actual == expected, (label, expected, actual)
    print(json.dumps({'phase': label, 'trusted': trusted, 'untrusted': untrusted}), flush=True)


created = False
try:
    kubectl('create', 'namespace', namespace)
    created = True
    for name in ('cn', 'trusted', 'untrusted'):
        apply(pod(name, name == 'trusted'))
    kubectl('-n', namespace, 'wait', '--for=condition=Ready', 'pod', '--all', '--timeout=30s')
    address = kubectl('-n', namespace, 'get', 'pod', 'cn', '-o', 'jsonpath={.status.podIP}')
    check('fresh-no-python-policy', address, [True, True, False], [True, True, False])
    tcp = [{'port': port, 'protocol': 'TCP'} for port in (6001,6002,6003,6004,6005,6006,7001)]
    apply(policy('platform', [{'from': [{'podSelector': {'matchLabels': {'trusted': 'true'}}}],
                               'ports': tcp + [{'port': 6005, 'protocol': 'UDP'}]}]))
    check('platform-preserved', address, [True, True, False], [False, False, False])
    apply(policy('legacy', [{'ports': tcp}]))
    check('legacy-union-counterexample', address, [True, True, False], [True, False, False])
    apply(policy('legacy', []))
    check('migrated-zero-grant-anchor', address, [True, True, False], [False, False, False])
    kubectl('-n', namespace, 'delete', 'networkpolicy', 'platform')
    check('platform-deletion-stays-isolated', address, [False, False, False], [False, False, False])
    print('PASS: TCP, UDP/6005, union, retained isolation, loopback exposure', flush=True)
finally:
    if created:
        kubectl('delete', 'namespace', namespace, '--wait=false')
