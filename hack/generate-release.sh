#!/bin/bash

set -euo pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
RELEASE_DIR="${REPO_ROOT}/out"
FLAVORS_DIR="${REPO_ROOT}/templates/flavors"
# Passed in by the Makefile (tracks KUSTOMIZE_VERSION); fallback for manual runs.
KUSTOMIZE="${KUSTOMIZE:-${REPO_ROOT}/bin/kustomize-v5.8.1}"

SUPPORTED_FLAVORS=(
    "default"
    "cilium"
    "vultr-ccm"
    "vultr-csi"
    "full"
    "bare-metal"
    "bare-metal-cp"
)

mkdir -p "${RELEASE_DIR}"

# ClusterClass definitions: each file carries the class and every template it
# references (clusterctl contract), named clusterclass-<class name>.yaml.
echo "==> Building ClusterClasses..."
"${KUSTOMIZE}" build "${REPO_ROOT}/templates/base" > "${RELEASE_DIR}/clusterclass-vultr.yaml"
"${KUSTOMIZE}" build "${FLAVORS_DIR}/bare-metal/clusterclass" > "${RELEASE_DIR}/clusterclass-vultr-bare-metal.yaml"
"${KUSTOMIZE}" build "${FLAVORS_DIR}/bare-metal-cp/clusterclass" > "${RELEASE_DIR}/clusterclass-vultr-bare-metal-cp.yaml"

echo "==> Copying standalone cluster template..."
cp "${REPO_ROOT}/templates/cluster-template.yaml" "${RELEASE_DIR}/cluster-template.yaml"
cp "${REPO_ROOT}/templates/cluster-template-bare-metal-standalone.yaml" "${RELEASE_DIR}/cluster-template-bare-metal-standalone.yaml"
cp "${REPO_ROOT}/templates/cluster-template-kamaji.yaml" "${RELEASE_DIR}/cluster-template-kamaji.yaml"

echo "==> Building infrastructure components..."
"${KUSTOMIZE}" build "${REPO_ROOT}/config/default" > "${RELEASE_DIR}/infrastructure-components.yaml"

for flavor in "${SUPPORTED_FLAVORS[@]}"; do
    echo "==> Generating ${flavor} flavor..."
    "${KUSTOMIZE}" build "${FLAVORS_DIR}/${flavor}" > "${RELEASE_DIR}/cluster-template-${flavor}.yaml"
done

mv "${RELEASE_DIR}/cluster-template-default.yaml" "${RELEASE_DIR}/cluster-template-clusterclass-kubeadm.yaml"

echo "==> Done. Release artifacts in ${RELEASE_DIR}/"
echo "    - infrastructure-components.yaml"
echo "    - clusterclass-vultr.yaml"
echo "    - clusterclass-vultr-bare-metal.yaml"
echo "    - clusterclass-vultr-bare-metal-cp.yaml"
echo "    - cluster-template.yaml"
echo "    - cluster-template-clusterclass-kubeadm.yaml"
echo "    - cluster-template-full.yaml"
echo "    - cluster-template-cilium.yaml"
echo "    - cluster-template-vultr-ccm.yaml"
echo "    - cluster-template-vultr-csi.yaml"
echo "    - cluster-template-bare-metal.yaml"
echo "    - cluster-template-bare-metal-standalone.yaml"
echo "    - cluster-template-kamaji.yaml"
echo "    - cluster-template-bare-metal-cp.yaml"