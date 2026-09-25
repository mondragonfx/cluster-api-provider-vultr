package controller

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "github.com/vultr/cluster-api-provider-vultr/api/v1beta2"
	"github.com/vultr/cluster-api-provider-vultr/cloud/scope"
)

// An externally managed control plane publishes the endpoint itself, so the
// provider waits for it instead of creating a load balancer. This path never
// calls the Vultr API, so it needs no client.
func TestReconcileExternalControlPlaneEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      clusterv1.APIEndpoint
		wantRequeue   bool
		wantProvision bool
		wantReason    string
	}{
		{
			name:        "waits while the endpoint is empty",
			endpoint:    clusterv1.APIEndpoint{},
			wantRequeue: true,
			wantReason:  infrav1.WaitingForExternalEndpointReason,
		},
		{
			name:        "waits while only the host is set",
			endpoint:    clusterv1.APIEndpoint{Host: "10.0.0.1"},
			wantRequeue: true,
			wantReason:  infrav1.WaitingForExternalEndpointReason,
		},
		{
			name:          "ready once host and port are set",
			endpoint:      clusterv1.APIEndpoint{Host: "10.0.0.1", Port: 6443},
			wantProvision: true,
			wantReason:    "ExternalControlPlaneEndpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vultrCluster := &infrav1.VultrCluster{
				Spec: infrav1.VultrClusterSpec{
					Network: infrav1.NetworkSpec{
						APIServerLoadbalancers: infrav1.VultrLoadBalancer{Enabled: ptr.To(false)},
					},
					ControlPlaneEndpoint: tt.endpoint,
				},
			}
			clusterScope := &scope.ClusterScope{Logger: logr.Discard(), VultrCluster: vultrCluster}
			if clusterScope.IsLoadBalancerEnabled() {
				t.Fatal("load balancer should be reported as disabled")
			}

			r := &VultrClusterReconciler{}
			res, err := r.reconcileExternalControlPlaneEndpoint(clusterScope)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantRequeue && res.RequeueAfter != 15*time.Second {
				t.Errorf("RequeueAfter = %v, want 15s", res.RequeueAfter)
			}
			if !tt.wantRequeue && res.RequeueAfter != 0 {
				t.Errorf("RequeueAfter = %v, want 0", res.RequeueAfter)
			}

			if got := ptr.Deref(vultrCluster.Status.Initialization.Provisioned, false); got != tt.wantProvision {
				t.Errorf("provisioned = %v, want %v", got, tt.wantProvision)
			}
			if vultrCluster.Status.Ready != tt.wantProvision {
				t.Errorf("status.ready = %v, want %v", vultrCluster.Status.Ready, tt.wantProvision)
			}

			c := conditions.Get(vultrCluster, infrav1.LoadBalancerReadyCondition)
			if c == nil {
				t.Fatal("LoadBalancerReady condition was not set")
			}
			if c.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", c.Reason, tt.wantReason)
			}
			wantStatus := metav1.ConditionFalse
			if tt.wantProvision {
				wantStatus = metav1.ConditionTrue
			}
			if c.Status != wantStatus {
				t.Errorf("status = %q, want %q", c.Status, wantStatus)
			}
		})
	}
}

// IsLoadBalancerEnabled defaults to true so existing clusters keep their load balancer.
func TestIsLoadBalancerEnabledDefaultsTrue(t *testing.T) {
	clusterScope := &scope.ClusterScope{VultrCluster: &infrav1.VultrCluster{}}
	if !clusterScope.IsLoadBalancerEnabled() {
		t.Error("an unset Enabled field should mean the load balancer is managed")
	}
}
