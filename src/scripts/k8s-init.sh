#!/bin/bash

# MYIP=$(ip a s ens3 | awk -F"[/ ]+" '/inet / {print $3}') && ./k8s-init.sh 10.244.0.0/16 10.96.0.0/12 cluster.local $MYIP
# When run this script in k8s nodes, edit controlPlaneEndpoint's port(9998)

# kubeadm-config 정의
# - controlPlaneEndpoint 에 LB 지정 (9998 포트)
# - advertise-address 에 Public IP 지정
cat << EOF > kubeadm-config.yaml
apiVersion: kubeadm.k8s.io/v1beta2
kind: InitConfiguration
nodeRegistration:
  kubeletExtraArgs:
    cloud-provider: "external"
---
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
imageRepository: k8s.gcr.io
controlPlaneEndpoint: $4:9998
dns:
  type: CoreDNS
apiServer:
  extraArgs:
    advertise-address: $4
    authorization-mode: Node,RBAC
etcd:
  local:
    dataDir: /var/lib/etcd
networking:
  dnsDomain: $3
  podSubnet: $1
  serviceSubnet: $2
controllerManager: {}
scheduler: {}
EOF

# Control-plane init
sudo kubeadm init --v=5 --upload-certs --config kubeadm-config.yaml

# control-plane leader 의 경우
# - mcks-bootstrap 데몬이 자동 실행
#systemctl status mcks-bootstrap
