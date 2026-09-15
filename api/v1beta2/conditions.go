package v1beta2

// Condition types
const (
	VultrClusterReadyCondition string = "Ready"
	LoadBalancerReadyCondition string = "LoadBalancerReady"

	ErrorFetchingLBReason          string = "ErrorFetchingLoadBalancer"
	ErrorCreatingLBReason          string = "ErrorCreatingLoadBalancer"
	LoadBalancerProvisioningFailed string = "LoadBalancerProvisioningFailed"
	WaitingForIPReason             string = "WaitingForIP"
)

// VultrBareMetalMachine condition types and reasons.
const (
	// BareMetalMachineReadyCondition summarizes the state of the machine infrastructure. Cluster API
	// mirrors it into the owning Machine's InfrastructureReady condition (v1beta2 contract).
	BareMetalMachineReadyCondition string = "Ready"
	// BareMetalServerActiveCondition reports whether the Vultr bare metal server is active.
	BareMetalServerActiveCondition string = "ServerActive"
	// BareMetalVPCAttachedCondition reports whether the requested VPC is attached to the server.
	BareMetalVPCAttachedCondition string = "VPCAttached"
	// BareMetalLoadBalancerMemberCondition reports whether a control plane server is a
	// backend of the cluster's API server load balancer.
	BareMetalLoadBalancerMemberCondition string = "LoadBalancerMember"

	BareMetalServerPendingReason          string = "ServerPending"
	BareMetalServerActiveReason           string = "ServerActive"
	BareMetalWaitingForServerActiveReason string = "WaitingForServerActive"
	BareMetalVPCAttachRequestedReason     string = "VPCAttachRequested"
	BareMetalVPCAttachedReason            string = "VPCAttached"
	BareMetalWaitingForLoadBalancerReason string = "WaitingForLoadBalancer"
	BareMetalLoadBalancerMemberReason     string = "LoadBalancerMember"

	// Terminal reasons: the machine will not be reconciled further and should be
	// replaced (for example by a MachineHealthCheck).
	BareMetalServerCreationFailedReason string = "ServerCreationFailed"
	BareMetalServerNotFoundReason       string = "ServerNotFound"
	BareMetalUnexpectedStatusReason     string = "UnexpectedStatus"
	BareMetalVPCAttachFailedReason      string = "VPCAttachFailed"
)
