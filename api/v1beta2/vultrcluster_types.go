/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// ClusterFinalizer allows ReconcileVultrCluster to clean up Vultr resources associated with VultrCluster
	// before removing it from the apiserver.
	ClusterFinalizer = "vultrcluster.infrastructure.cluster.x-k8s.io"
)

// VultrClusterSpec defines the desired state of VultrCluster.
type VultrClusterSpec struct {
	// The Vultr Region of the cluster
	Region string `json:"region"`

	// NetworkSpec encapsulates all things related to Vultr network.
	// +optional
	Network NetworkSpec `json:"network,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`
}

// VultrClusterStatus defines the observed state of VultrCluster (v1beta2 style).
type VultrClusterStatus struct {
	// Initialization provides observations of the Cluster initialization process.
	// +optional
	Initialization ClusterInitializationStatus `json:"initialization,omitempty"`

	// Represents the observations of a Cluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ControlPlane groups all the observations about Cluster's ControlPlane current state.
	// +optional
	ControlPlane *ClusterControlPlaneStatus `json:"controlPlane,omitempty"`

	// Workers groups all the observations about Cluster's Workers current state.
	// +optional
	Workers *WorkersStatus `json:"workers,omitempty"`

	// Network encapsulates all things related to the Vultr network.
	// +optional
	Network VultrNetworkResource `json:"network,omitempty"`
}

// ClusterInitializationStatus provides observations of the Cluster initialization process.
type ClusterInitializationStatus struct {
	// InfrastructureProvisioned is true when the infrastructure provider reports
	// that Cluster's infrastructure is fully provisioned.
	// +optional
	InfrastructureProvisioned bool `json:"infrastructureProvisioned,omitempty"`

	// ControlPlaneInitialized denotes when the control plane is functional enough to accept requests.
	// +optional
	ControlPlaneInitialized bool `json:"controlPlaneInitialized,omitempty"`
}

// ClusterControlPlaneStatus groups all the observations about control plane current state.
type ClusterControlPlaneStatus struct {
	DesiredReplicas   *int32 `json:"desiredReplicas,omitempty"`
	Replicas          *int32 `json:"replicas,omitempty"`
	UpToDateReplicas  *int32 `json:"upToDateReplicas,omitempty"`
	ReadyReplicas     *int32 `json:"readyReplicas,omitempty"`
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`
}

// WorkersStatus groups all the observations about workers current state.
type WorkersStatus struct {
	DesiredReplicas   *int32 `json:"desiredReplicas,omitempty"`
	Replicas          *int32 `json:"replicas,omitempty"`
	UpToDateReplicas  *int32 `json:"upToDateReplicas,omitempty"`
	ReadyReplicas     *int32 `json:"readyReplicas,omitempty"`
	AvailableReplicas *int32 `json:"availableReplicas,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=vultrclusters,scope=Namespaced,categories=cluster-api
//+kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this VultrCluster belongs"
//+kubebuilder:printcolumn:name="Available",type="string",JSONPath=".status.conditions[?(@.type==\"Available\")].status",description="Cluster infrastructure availability"
//+kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint",description="API Endpoint",priority=1

// VultrCluster is the Schema for the vultrclusters API.
type VultrCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VultrClusterSpec   `json:"spec,omitempty"`
	Status VultrClusterStatus `json:"status,omitempty"`
}

// GetConditions returns the list of conditions for a VultrCluster.
func (r *VultrCluster) GetConditions() []metav1.Condition {
	return r.Status.Conditions
}

// SetConditions sets the conditions on a VultrCluster.
func (r *VultrCluster) SetConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

//+kubebuilder:object:root=true

// VultrClusterList contains a list of VultrCluster.
type VultrClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VultrCluster `json:"items"`
}

// Ensure VultrCluster implements the conditions interfaces.
var (
	_ conditions.Getter = &VultrCluster{}
	_ conditions.Setter = &VultrCluster{}
)

func init() {
	SchemeBuilder.Register(&VultrCluster{}, &VultrClusterList{})
}
