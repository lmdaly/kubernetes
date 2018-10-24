# PoC NUMA Manager for Kubernetes

## Overview

NUMA Manager for Kubernetes co-ordinates NUMA Node locaility for devices managed by device plugins, CPUs managed by the CPU Manager.

This implementation is based on the following proposal: https://github.com/kubernetes/community/pull/1680


This is a PoC/Hack to faciliate the creation of a demo and start discussion within the community on this feature.

Feedback is welcome!

## How To

*Pre-Requisits*: A Kubernetes Cluster

1. Clone the repo & checkout branch
 `git clone https://github.com/lmdaly/kubernetes.git && git checkout dev/numa_manager`
2. Stop all the Kubernetes services
 `systemctl stop kube-apiserver kube-scheduler kube-controller-manager kube-proxy kubelet`

3. Build the various components for the top /kubernetes folder
 `make`

4. Copy the binaries to the relevant folder - most likely /usr/bin
 `cd _output/local/bin/linux/amd64 && yes "yes " | cp kubelet kube-apiserver kube-controller-manager  kube-scheduler kubectl kube-proxy kubemark hyperkube /usr/bin`

 5. Restart the services
 `systemctl start kube-apiserver kube-scheduler kube-controller-manager kube-proxy kubelet` 
 
6. Enable static policy of the CPU Manager in Kubelet
 *Note*: This is a Kubelet flag, make changes to KUBELET_ARGS
 `--feature-gates=CPUManager=true`
 `--cpu-manager-policy=static`

7. Reserve CPUs for System and Kube in Kubelet (required for static policy)
 *Note*: This is a Kubelet flag, make changes to KUBELET_ARGS
`--kube-reserved=cpu=1 --system-reserved=cpu=1`

8. Enable device plguins in kubelet
 *Note*: This is a Kubelet flag, make changes to KUBELET_ARGS
 `--feature-gates=DevicePlugins=True`

9. Create a Guaranteed Pod requesting CPU and Memory

10. Search for *numamanager* in the Kubelet logs
*Logs can be output to a file by editing the KUBE_LOGTOSTDERR flag with "--logtostderr=false --log-dir=/var/logs/kubernetes"*


## Detailed Document

https://docs.google.com/document/d/1RZsB-VEcoiN_Wbkx06OmmwyYTflV_XH-Ct_GUjahzVc/edit#heading=h.bm4jeiohhhav

## Design Proposal
https://github.com/kubernetes/community/pull/1680

## Hardware Topology Discussion Document

https://docs.google.com/document/d/1dv3-se21ivEZhyRCr9qixvjyB_VRO2yIlotWb2kOdiw/edit?ts=59e4be43#heading=h.t3uto9lxpu15


## Hardware Topology GitHub Issue

https://github.com/kubernetes/kubernetes/issues/4996
