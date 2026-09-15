/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/vultr/govultr/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/vultr/cluster-api-provider-vultr/api/v1beta2"
	"github.com/vultr/cluster-api-provider-vultr/cloud/scope"
	"github.com/vultr/cluster-api-provider-vultr/cloud/services"
	"github.com/vultr/cluster-api-provider-vultr/util/reconciler"
)

const (
	// bareMetalPendingRequeue is how often a provisioning bare metal server is polled.
	// Bare metal provisioning typically takes 10-15 minutes.
	bareMetalPendingRequeue = 30 * time.Second
	// bareMetalVPCRequeue is how often a requested VPC attachment is re-checked.
	bareMetalVPCRequeue = 15 * time.Second
	// bareMetalDeleteRequeue is how often deletion is retried while the server is still provisioning.
	bareMetalDeleteRequeue = 30 * time.Second
)

// VultrBareMetalMachineReconciler reconciles a VultrBareMetalMachine object.
type VultrBareMetalMachineReconciler struct {
	client.Client
	Recorder         events.EventRecorder
	ReconcileTimeout time.Duration
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vultrbaremetalmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vultrbaremetalmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=vultrbaremetalmachines/finalizers,verbs=update

func (r *VultrBareMetalMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultedLoopTimeout(r.ReconcileTimeout))
	defer cancel()

	log := ctrl.LoggerFrom(ctx)

	bareMetalMachine := &v1beta2.VultrBareMetalMachine{}
	if err := r.Get(ctx, req.NamespacedName, bareMetalMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	machine, err := util.GetOwnerMachine(ctx, r.Client, bareMetalMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	vultrCluster := &v1beta2.VultrCluster{}
	vultrClusterName := client.ObjectKey{
		Namespace: bareMetalMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Get(ctx, vultrClusterName, vultrCluster); err != nil && !scope.MachineOnly() {
		log.Info("VultrCluster is not available yet.")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, vultrCluster) && !scope.MachineOnly() {
		log.Info("VultrBareMetalMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:       r.Client,
		Logger:       log,
		Cluster:      cluster,
		VultrCluster: vultrCluster,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %v", err)
	}

	machineScope, err := scope.NewBareMetalMachineScope(scope.BareMetalMachineScopeParams{
		Client:                r.Client,
		Logger:                log,
		Cluster:               cluster,
		Machine:               machine,
		VultrCluster:          vultrCluster,
		VultrBareMetalMachine: bareMetalMachine,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create bare metal machine scope: %v", err)
	}

	defer func() {
		if err := machineScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !bareMetalMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope, clusterScope)
	}

	return r.reconcileNormal(ctx, machineScope, clusterScope)
}

func (r *VultrBareMetalMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.BareMetalMachineScope, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling VultrBareMetalMachine")
	bareMetalMachine := machineScope.VultrBareMetalMachine

	if machineScope.HasTerminalFailure() {
		machineScope.Info("Machine infrastructure has failed, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	controllerutil.AddFinalizer(bareMetalMachine, v1beta2.BareMetalMachineFinalizer)

	if !conditions.IsTrue(machineScope.Cluster, clusterv1.InfrastructureReadyCondition) {
		machineScope.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		machineScope.Info("Bootstrap data secret reference is not yet available")
		return reconcile.Result{}, nil
	}

	if machineScope.IsControlPlane() {
		msg := "bare metal control plane machines are not supported; use a VultrMachineTemplate for the control plane"
		r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeWarning, "ControlPlaneNotSupported", "Rejected", msg)
		machineScope.SetTerminalFailure(v1beta2.BareMetalControlPlaneNotSupportedReason, msg)
		return reconcile.Result{}, nil
	}

	svc := services.NewService(ctx, clusterScope)

	server, err := r.findOrCreateServer(ctx, svc, machineScope)
	if err != nil || server == nil {
		return reconcile.Result{}, err
	}

	machineScope.SetProviderID(server.ID)
	machineScope.SetServerStatus(v1beta2.SubscriptionStatus(server.Status))
	machineScope.SetHardware(server.CPUCount, server.RAM, server.Disk)

	switch v1beta2.SubscriptionStatus(server.Status) {
	case v1beta2.SubscriptionStatusPending:
		machineScope.Info("Bare metal server is provisioning", "server-id", server.ID)
		conditions.Set(bareMetalMachine, metav1.Condition{
			Type:               v1beta2.BareMetalServerActiveCondition,
			Status:             metav1.ConditionFalse,
			Reason:             v1beta2.BareMetalServerPendingReason,
			Message:            "Bare metal server is provisioning",
			LastTransitionTime: metav1.Now(),
		})
		conditions.Set(bareMetalMachine, metav1.Condition{
			Type:               v1beta2.BareMetalMachineReadyCondition,
			Status:             metav1.ConditionFalse,
			Reason:             v1beta2.BareMetalServerPendingReason,
			Message:            "Bare metal server is provisioning",
			LastTransitionTime: metav1.Now(),
		})
		return reconcile.Result{RequeueAfter: bareMetalPendingRequeue}, nil

	case v1beta2.SubscriptionStatusActive:
		conditions.Set(bareMetalMachine, metav1.Condition{
			Type:               v1beta2.BareMetalServerActiveCondition,
			Status:             metav1.ConditionTrue,
			Reason:             v1beta2.BareMetalServerActiveReason,
			Message:            "Bare metal server is active",
			LastTransitionTime: metav1.Now(),
		})

	default:
		msg := fmt.Sprintf("bare metal server status %q is unexpected", server.Status)
		machineScope.Info("Bare metal server status is unexpected", "server-id", server.ID, "status", server.Status)
		r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeWarning, "UnexpectedStatus", "Failed", msg)
		machineScope.SetTerminalFailure(v1beta2.BareMetalUnexpectedStatusReason, msg)
		return reconcile.Result{}, nil
	}

	var vpcInfo *govultr.VPCInfo
	if vpcID := bareMetalMachine.Spec.VPCID; vpcID != "" {
		attached, info, err := svc.EnsureBareMetalVPC(server.ID, vpcID)
		if err != nil {
			if services.IsTerminalVPCAttachError(err) {
				r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeWarning, "VPCAttachFailed", "Failed", err.Error())
				conditions.Set(bareMetalMachine, metav1.Condition{
					Type:               v1beta2.BareMetalVPCAttachedCondition,
					Status:             metav1.ConditionFalse,
					Reason:             v1beta2.BareMetalVPCAttachFailedReason,
					Message:            err.Error(),
					LastTransitionTime: metav1.Now(),
				})
				machineScope.SetTerminalFailure(v1beta2.BareMetalVPCAttachFailedReason, err.Error())
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}
		if !attached {
			machineScope.Info("Waiting for VPC attachment", "server-id", server.ID, "vpc-id", vpcID)
			conditions.Set(bareMetalMachine, metav1.Condition{
				Type:               v1beta2.BareMetalVPCAttachedCondition,
				Status:             metav1.ConditionFalse,
				Reason:             v1beta2.BareMetalVPCAttachRequestedReason,
				Message:            "VPC attachment requested",
				LastTransitionTime: metav1.Now(),
			})
			conditions.Set(bareMetalMachine, metav1.Condition{
				Type:               v1beta2.BareMetalMachineReadyCondition,
				Status:             metav1.ConditionFalse,
				Reason:             v1beta2.BareMetalVPCAttachRequestedReason,
				Message:            "Waiting for VPC attachment",
				LastTransitionTime: metav1.Now(),
			})
			return reconcile.Result{RequeueAfter: bareMetalVPCRequeue}, nil
		}
		vpcInfo = info
		conditions.Set(bareMetalMachine, metav1.Condition{
			Type:               v1beta2.BareMetalVPCAttachedCondition,
			Status:             metav1.ConditionTrue,
			Reason:             v1beta2.BareMetalVPCAttachedReason,
			Message:            "VPC is attached",
			LastTransitionTime: metav1.Now(),
		})
		machineScope.SetVPC(&v1beta2.VultrBareMetalVPCStatus{
			ID:         info.ID,
			IPAddress:  info.IPAddress,
			MACAddress: info.MacAddress,
		})
	}

	machineScope.SetAddresses(services.GetBareMetalServerAddresses(server, vpcInfo))

	conditions.Set(bareMetalMachine, metav1.Condition{
		Type:               v1beta2.BareMetalMachineReadyCondition,
		Status:             metav1.ConditionTrue,
		Reason:             v1beta2.BareMetalServerActiveReason,
		Message:            "Bare metal server is active",
		LastTransitionTime: metav1.Now(),
	})
	machineScope.SetReady()
	machineScope.SetProvisioned()

	return reconcile.Result{}, nil
}

// findOrCreateServer returns the bare metal server for the machine, locating it by
// providerID, then by tags (a server whose id was never persisted), and finally
// creating it. It returns nil, nil after recording a terminal failure.
func (r *VultrBareMetalMachineReconciler) findOrCreateServer(ctx context.Context, svc *services.Service, machineScope *scope.BareMetalMachineScope) (*govultr.BareMetalServer, error) {
	bareMetalMachine := machineScope.VultrBareMetalMachine

	if id := machineScope.GetServerID(); id != "" {
		server, err := svc.GetBareMetalServer(id)
		if err != nil {
			return nil, err
		}
		if server == nil {
			msg := fmt.Sprintf("bare metal server %s no longer exists", id)
			r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeWarning, "ServerNotFound", "Failed", msg)
			machineScope.SetTerminalFailure(v1beta2.BareMetalServerNotFoundReason, msg)
			return nil, nil
		}
		return server, nil
	}

	server, err := svc.FindBareMetalServerByTags(machineScope.Name(), machineScope.Role())
	if err != nil {
		return nil, err
	}
	if server != nil {
		r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeNormal, "ServerAdopted", "Adopted",
			"Adopted existing bare metal server %s", server.ID)
		return server, nil
	}

	server, err = svc.CreateBareMetalServer(ctx, machineScope)
	if err != nil {
		r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeWarning, "ServerCreationFailed", "Failed",
			"Failed to create bare metal server: %v", err)
		if isTerminalCreateError(err) {
			machineScope.SetTerminalFailure(v1beta2.BareMetalServerCreationFailedReason, err.Error())
			return nil, nil
		}
		return nil, err
	}
	r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeNormal, "ServerCreated", "Created",
		"Created bare metal server %s (%s)", server.ID, server.Plan)

	return server, nil
}

// vultrAPIErrorStatus extracts the HTTP status from a Vultr API error. govultr
// surfaces API failures as errors whose text is the JSON body returned by the
// API (for example {"error":"...","status":422}); transport errors carry no status.
func vultrAPIErrorStatus(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	m := vultrAPIStatusRe.FindStringSubmatch(err.Error())
	if m == nil {
		return 0, false
	}
	status, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return 0, false
	}
	return status, true
}

var vultrAPIStatusRe = regexp.MustCompile(`"status"\s*:\s*(\d{3})`)

// isTerminalCreateError reports whether a create failure will keep failing on
// retry: the API rejected the request (4xx) rather than failing transiently.
// 422 is excluded because Vultr uses it for "still activating" style responses.
func isTerminalCreateError(err error) bool {
	status, ok := vultrAPIErrorStatus(err)
	return ok && status >= 400 && status < 500 && status != http.StatusUnprocessableEntity
}

func (r *VultrBareMetalMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.BareMetalMachineScope, clusterScope *scope.ClusterScope) (reconcile.Result, error) {
	machineScope.Info("Reconciling delete VultrBareMetalMachine")
	bareMetalMachine := machineScope.VultrBareMetalMachine

	svc := services.NewService(ctx, clusterScope)

	var server *govultr.BareMetalServer
	var err error
	if id := machineScope.GetServerID(); id != "" {
		server, err = svc.GetBareMetalServer(id)
	} else {
		server, err = svc.FindBareMetalServerByTags(machineScope.Name(), machineScope.Role())
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	if server == nil {
		r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeNormal, "NoServerFound", "NotFound", "No bare metal server to delete")
		controllerutil.RemoveFinalizer(bareMetalMachine, v1beta2.BareMetalMachineFinalizer)
		return reconcile.Result{}, nil
	}

	if err := svc.DeleteBareMetalServer(server.ID); err != nil {
		if _, isAPIError := vultrAPIErrorStatus(err); isAPIError {
			// Vultr refuses to delete a server that is still provisioning; retry later.
			r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeWarning, "ServerDeleteRetry", "Retrying",
				"Bare metal server %s could not be deleted yet: %v", server.ID, err)
			return reconcile.Result{RequeueAfter: bareMetalDeleteRequeue}, nil
		}
		return reconcile.Result{}, err
	}

	r.Recorder.Eventf(bareMetalMachine, nil, corev1.EventTypeNormal, "ServerDeleted", "Deleted", "Deleted bare metal server %s", server.ID)
	controllerutil.RemoveFinalizer(bareMetalMachine, v1beta2.BareMetalMachineFinalizer)
	return reconcile.Result{}, nil
}

func (r *VultrBareMetalMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, _ controller.Options) error {
	clusterToObjectFunc, err := util.ClusterToTypedObjectsMapper(r.Client, &v1beta2.VultrBareMetalMachineList{}, mgr.GetScheme())
	if err != nil {
		return errors.Wrapf(err, "failed to create mapper for Cluster to VultrBareMetalMachines")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.VultrBareMetalMachine{}).
		WithEventFilter(predicates.ResourceNotPaused(mgr.GetScheme(), ctrl.LoggerFrom(ctx))).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(v1beta2.GroupVersion.WithKind("VultrBareMetalMachine"))),
		).
		Watches(
			&v1beta2.VultrCluster{},
			handler.EnqueueRequestsFromMapFunc(r.VultrClusterToVultrBareMetalMachines(ctx)),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToObjectFunc),
			builder.WithPredicates(predicates.ClusterPausedTransitionsOrInfrastructureProvisioned(mgr.GetScheme(), ctrl.LoggerFrom(ctx))),
		).
		Complete(r)
}

// VultrClusterToVultrBareMetalMachines enqueues the VultrBareMetalMachines of the cluster owning a VultrCluster.
func (r *VultrBareMetalMachineReconciler) VultrClusterToVultrBareMetalMachines(ctx context.Context) handler.MapFunc {
	log := ctrl.LoggerFrom(ctx)
	return func(ctx context.Context, o client.Object) []ctrl.Request {
		result := []ctrl.Request{}

		c, ok := o.(*v1beta2.VultrCluster)
		if !ok {
			log.Error(errors.Errorf("expected a VultrCluster but got a %T", o), "failed to get VultrBareMetalMachines for VultrCluster")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			return result
		case err != nil:
			log.Error(err, "failed to get owning cluster")
			return result
		}

		labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
		machineList := &clusterv1.MachineList{}
		if err := r.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			log.Error(err, "failed to list Machines")
			return nil
		}
		for _, m := range machineList.Items {
			if m.Spec.InfrastructureRef.Name == "" || m.Spec.InfrastructureRef.Kind != "VultrBareMetalMachine" {
				continue
			}
			result = append(result, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}})
		}

		return result
	}
}
