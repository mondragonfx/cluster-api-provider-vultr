# Bare metal worker nodes

CAPVULTR can run worker nodes on [Vultr Bare Metal](https://www.vultr.com/products/bare-metal/)
servers through the `VultrBareMetalMachine` / `VultrBareMetalMachineTemplate` kinds.
Both worker and control plane machines can run on bare metal. Control plane servers are
registered as backends of the cluster's Vultr load balancer like cloud compute control plane
instances; the `bare-metal-cp` flavor runs the whole cluster on bare metal.

## How it works

A `VultrBareMetalMachine` creates a bare metal server from a stock Vultr operating system
(`osID`, for example `2284` = Ubuntu 24.04) with the Cluster API bootstrap data as cloud-init
user data; Kubernetes is installed at first boot by the bootstrap configuration (see below).

Once the server is `active` the controller optionally attaches a VPC (`vpcID`), records the
addresses and marks the machine ready. Bare metal provisioning takes a few minutes longer
than a cloud instance (roughly 5–15 minutes depending on the plan), plus the package
installation.

## Prerequisites

- Bare metal must be enabled on your Vultr account.
- Pick a plan and region from `GET /v2/plans-metal` (`vultr-cli plans metal`). Bare metal plans
  are available in far fewer regions than cloud plans, and a plan can be temporarily out of
  stock in a region.
- VPC networking is only supported on some bare metal plans (for example `vbm-6c-32gb-amd`,
  `vbm-8c-132gb` and `vbm-24c-256gb-amd` accept a VPC, `vbm-4c-32gb` does not). The `bare-metal` flavor therefore
  runs **without a VPC by default**: every node (control plane and bare metal) uses its public
  IP and Cilium tunnels over it. On plans that support VPC networking you can set both `VPC_ID`
  (control plane and load balancer) and `BARE_METAL_VPC_ID` (bare metal workers) so all nodes
  share the VPC. Do not set only one of them: Cilium would try to reach the control plane on its
  VPC address from nodes that are not in the VPC. Setting `BARE_METAL_VPC_ID` on a plan without
  VPC support fails the machine with the `VPCAttachFailed` reason
  (`Plan does not support VPC networking`).
- Vultr's stock operating system images ship with `ufw` enabled (SSH only). The bootstrap data
  disables it so the kubelet, Cilium and pod traffic are reachable; when bringing your own
  bootstrap configuration, make sure the firewall allows cluster traffic.
- The [Vultr cloud controller manager](https://github.com/vultr/vultr-cloud-controller-manager)
  must run in the workload cluster. It looks bare metal nodes up through the bare metal API
  **only** when the node carries the label `vultr.com/baremetal=true`; the templates below set
  it through the kubelet `--node-labels` argument. Without the label the CCM treats the node as
  a missing cloud instance and removes it.

## Tested plans

The provider does not depend on the plan; the operating system does. Verified so far with
Ubuntu 24.04: `vbm-4c-32gb` (Intel,
BIOS boot, no VPC), `vbm-6c-32gb-amd`, `vbm-8c-132gb` and `vbm-24c-256gb-amd` (UEFI boot, two
NICs, NVMe disks, VPC capable). Plans can be temporarily out of stock in a region even though
`/v2/plans-metal` lists the region; the machine then fails with `ServerCreationFailed`.

## Templates

`cluster-template-bare-metal.yaml` (`--flavor bare-metal`) is a ClusterClass based template
with a cloud compute control plane, an optional cloud compute worker pool (`md-0`, default 0
replicas) and a bare metal worker pool (`bm-0`). It includes Cilium (through the Cluster API
Helm addon provider) and the Vultr CCM. The management cluster needs
`CLUSTER_TOPOLOGY=true` and `--addon helm` when running `clusterctl init`.

```sh
export CLUSTER_NAME=bm-demo
export KUBERNETES_VERSION=v1.34.3
export REGION=lax
export CONTROL_PLANE_PLAN_ID=vc2-2c-4gb
export WORKER_PLAN_ID=vc2-2c-4gb
export MACHINE_IMAGE=<snapshot id of a Cluster API image for the control plane>
export SSH_KEY_ID=<vultr ssh key id>
export BARE_METAL_PLAN_ID=vbm-4c-32gb
export BARE_METAL_OS_ID=2284
export BARE_METAL_WORKER_MACHINE_COUNT=1

clusterctl generate cluster ${CLUSTER_NAME} --flavor bare-metal | kubectl apply -f -
```

To put the cluster on a VPC, add the `VPC_ID` and `BARE_METAL_VPC_ID` variables to the
generated Cluster's `spec.topology.variables` (both default to empty in the ClusterClass).

`cluster-template-bare-metal-cp.yaml` (`--flavor bare-metal-cp`) runs the control plane on bare
metal as well (ClusterClass `vultr-bare-metal-cp`): the control plane servers use
`BARE_METAL_CONTROL_PLANE_PLAN_ID` (default `vbm-4c-32gb`), the same operating system as
the workers, and are registered as backends of the Vultr load balancer.
Keep in mind that a bare metal control plane takes several minutes longer to provision or
replace than a cloud compute one.

`cluster-template-bare-metal-standalone.yaml` is the same layout without ClusterClass
(plain `MachineDeployment` / `KubeadmConfigTemplate` / `VultrBareMetalMachineTemplate`).

The `KubeadmConfigTemplate` ships `/usr/local/bin/install-k8s.sh`, which
installs containerd and the `kubelet`/`kubeadm`/`kubectl` packages for the cluster's
Kubernetes version from `pkgs.k8s.io` before `kubeadm join` runs. Only Ubuntu 24.04 is
covered by that script; adapt it for other operating systems.

## Watching a bare metal machine

```sh
kubectl get vbm            # short name for vultrbaremetalmachines
kubectl describe vbm <name>
```

Conditions:

| Condition      | Meaning                                                                    |
|----------------|----------------------------------------------------------------------------|
| `Ready`        | Summary; mirrored into the Machine's `InfrastructureReady`. `False` with a terminal reason (`ServerCreationFailed`, `ServerNotFound`, `UnexpectedStatus`, `VPCAttachFailed`) means the controller stopped reconciling the machine; replace the Machine (for example through a MachineHealthCheck or by scaling the MachineDeployment). |
| `ServerActive` | The Vultr server reports `active`.                                          |
| `VPCAttached`  | Only when `vpcID` is set: the VPC shows up on the server.                   |
| `LoadBalancerMember` | Control plane machines only: the server is a backend of the API server load balancer. |

Servers are tagged like cloud instances (`sigs-k8s-io:capvultr:<cluster>...`, `name:<machine>`);
the controller uses the tags to adopt a server whose id was not recorded yet, so a failed
reconcile never leaves an orphaned server behind.
