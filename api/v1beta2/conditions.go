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
	// BareMetalMachineReadyCondition summarizes the state of the machine infrastructure across its
	// whole lifecycle. Cluster API mirrors it into the owning Machine's InfrastructureReady
	// condition (v1beta2 contract); the reason tells the current phase.
	BareMetalMachineReadyCondition string = "Ready"

	BareMetalWaitingForClusterInfrastructureReason string = "WaitingForClusterInfrastructure"
	BareMetalWaitingForBootstrapDataReason         string = "WaitingForBootstrapData"
	BareMetalServerPendingReason                   string = "ServerPending"
	BareMetalVPCAttachRequestedReason              string = "VPCAttachRequested"
	BareMetalServerActiveReason                    string = "ServerActive"
	BareMetalDeletingReason                        string = "Deleting"

	// Terminal reasons: the machine will not be reconciled further and should be
	// replaced (for example by a MachineHealthCheck).
	BareMetalServerCreationFailedReason string = "ServerCreationFailed"
	BareMetalServerNotFoundReason       string = "ServerNotFound"
	BareMetalUnexpectedStatusReason     string = "UnexpectedStatus"
	BareMetalVPCAttachFailedReason      string = "VPCAttachFailed"
)
