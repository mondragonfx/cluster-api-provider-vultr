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
