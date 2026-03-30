/*
Copyright 2026 Erling Kristiansen.

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

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openclawv1alpha1 "github.com/egkristi/kubeclaw/api/v1alpha1"
)

const (
	openclawTenantFinalizer = "openclaw.io/tenant-finalizer"

	TenantPhasePending     = "Pending"
	TenantPhaseActive      = "Active"
	TenantPhaseTerminating = "Terminating"
	TenantPhaseFailed      = "Failed"
)

// OpenClawTenantReconciler reconciles a OpenClawTenant object
type OpenClawTenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=openclaw.io,resources=openclawtenants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openclaw.io,resources=openclawtenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openclaw.io,resources=openclawtenants/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile implements the reconciliation loop for tenants
func (r *OpenClawTenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	tenant := &openclawv1alpha1.OpenClawTenant{}
	if err := r.Get(ctx, req.NamespacedName, tenant); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Tenant resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get tenant")
		return ctrl.Result{}, err
	}

	// Set finalizer for cleanup
	if tenant.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(tenant, openclawTenantFinalizer) {
			controllerutil.AddFinalizer(tenant, openclawTenantFinalizer)
			if err := r.Update(ctx, tenant); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// Handle deletion
		if controllerutil.ContainsFinalizer(tenant, openclawTenantFinalizer) {
			if err := r.deleteTenantResources(ctx, tenant); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(tenant, openclawTenantFinalizer)
			if err := r.Update(ctx, tenant); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Create or update namespace
	if err := r.reconcileNamespace(ctx, tenant); err != nil {
		return r.setTenantPhase(ctx, tenant, TenantPhaseFailed, err)
	}

	// Create ResourceQuota
	if err := r.reconcileResourceQuota(ctx, tenant); err != nil {
		return r.setTenantPhase(ctx, tenant, TenantPhaseFailed, err)
	}

	// Create LimitRange
	if err := r.reconcileLimitRange(ctx, tenant); err != nil {
		return r.setTenantPhase(ctx, tenant, TenantPhaseFailed, err)
	}

	// Create NetworkPolicy for isolation
	if tenant.Spec.NetworkIsolation.Enabled {
		if err := r.reconcileNetworkPolicy(ctx, tenant); err != nil {
			return r.setTenantPhase(ctx, tenant, TenantPhaseFailed, err)
		}
	}

	// Update instance count
	instanceCount, err := r.countTenantInstances(ctx, tenant)
	if err != nil {
		logger.Error(err, "Failed to count tenant instances")
	}
	tenant.Status.InstanceCount = instanceCount

	return r.setTenantPhase(ctx, tenant, TenantPhaseActive, nil)
}

// reconcileNamespace creates or updates the tenant namespace
func (r *OpenClawTenantReconciler) reconcileNamespace(ctx context.Context, tenant *openclawv1alpha1.OpenClawTenant) error {
	ns := &corev1.Namespace{}
	ns.Name = tenant.Name

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		// Apply tenant labels
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		ns.Labels["openclaw.io/tenant"] = tenant.Name
		ns.Labels["openclaw.io/managed-by"] = "kubeclaw"

		for k, v := range tenant.Spec.Labels {
			ns.Labels[k] = v
		}

		// Apply annotations
		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations["openclaw.io/tenant-owner"] = tenant.Spec.Owner
		ns.Annotations["openclaw.io/description"] = tenant.Spec.Description

		for k, v := range tenant.Spec.Annotations {
			ns.Annotations[k] = v
		}

		return nil
	})

	return err
}

// reconcileResourceQuota creates ResourceQuota for the tenant
func (r *OpenClawTenantReconciler) reconcileResourceQuota(ctx context.Context, tenant *openclawv1alpha1.OpenClawTenant) error {
	if tenant.Spec.ResourceQuota.HardCPU.IsZero() &&
		tenant.Spec.ResourceQuota.HardMemory.IsZero() &&
		tenant.Spec.ResourceQuota.HardStorage.IsZero() {
		return nil // No quota specified
	}

	rq := &corev1.ResourceQuota{}
	rq.Name = fmt.Sprintf("%s-quota", tenant.Name)
	rq.Namespace = tenant.Name

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, rq, func() error {
		rq.Spec.Hard = corev1.ResourceList{}

		if !tenant.Spec.ResourceQuota.HardCPU.IsZero() {
			rq.Spec.Hard[corev1.ResourceCPU] = tenant.Spec.ResourceQuota.HardCPU
		}
		if !tenant.Spec.ResourceQuota.HardMemory.IsZero() {
			rq.Spec.Hard[corev1.ResourceMemory] = tenant.Spec.ResourceQuota.HardMemory
		}
		if !tenant.Spec.ResourceQuota.HardStorage.IsZero() {
			rq.Spec.Hard[corev1.ResourceStorage] = tenant.Spec.ResourceQuota.HardStorage
		}
		if tenant.Spec.ResourceQuota.HardPVCs > 0 {
			rq.Spec.Hard[corev1.ResourcePersistentVolumeClaims] = *resource.NewQuantity(
				int64(tenant.Spec.ResourceQuota.HardPVCs), resource.DecimalSI)
		}
		if tenant.Spec.ResourceQuota.HardServices > 0 {
			rq.Spec.Hard[corev1.ResourceServices] = *resource.NewQuantity(
				int64(tenant.Spec.ResourceQuota.HardServices), resource.DecimalSI)
		}
		if tenant.Spec.ResourceQuota.HardPods > 0 {
			rq.Spec.Hard[corev1.ResourcePods] = *resource.NewQuantity(
				int64(tenant.Spec.ResourceQuota.HardPods), resource.DecimalSI)
		}

		return nil
	})

	return err
}

// reconcileLimitRange creates LimitRange for default constraints
func (r *OpenClawTenantReconciler) reconcileLimitRange(ctx context.Context, tenant *openclawv1alpha1.OpenClawTenant) error {
	lr := &corev1.LimitRange{}
	lr.Name = fmt.Sprintf("%s-limits", tenant.Name)
	lr.Namespace = tenant.Name

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, lr, func() error {
		limits := []corev1.LimitRangeItem{}

		// Container limits
		containerLimits := corev1.LimitRangeItem{
			Type: corev1.LimitTypeContainer,
		}

		if !tenant.Spec.LimitRange.DefaultCPURequest.IsZero() {
			containerLimits.DefaultRequest[corev1.ResourceCPU] = tenant.Spec.LimitRange.DefaultCPURequest
		}
		if !tenant.Spec.LimitRange.DefaultCPULimit.IsZero() {
			containerLimits.Default[corev1.ResourceCPU] = tenant.Spec.LimitRange.DefaultCPULimit
		}
		if !tenant.Spec.LimitRange.DefaultMemoryRequest.IsZero() {
			containerLimits.DefaultRequest[corev1.ResourceMemory] = tenant.Spec.LimitRange.DefaultMemoryRequest
		}
		if !tenant.Spec.LimitRange.DefaultMemoryLimit.IsZero() {
			containerLimits.Default[corev1.ResourceMemory] = tenant.Spec.LimitRange.DefaultMemoryLimit
		}

		if len(containerLimits.Default) > 0 || len(containerLimits.DefaultRequest) > 0 {
			limits = append(limits, containerLimits)
		}

		// PVC limits
		pvcLimits := corev1.LimitRangeItem{
			Type: corev1.LimitTypePersistentVolumeClaim,
		}
		if !tenant.Spec.LimitRange.DefaultStorageRequest.IsZero() {
			pvcLimits.DefaultRequest[corev1.ResourceStorage] = tenant.Spec.LimitRange.DefaultStorageRequest
		}
		if len(pvcLimits.Default) > 0 || len(pvcLimits.DefaultRequest) > 0 {
			limits = append(limits, pvcLimits)
		}

		lr.Spec.Limits = limits
		return nil
	})

	return err
}

// reconcileNetworkPolicy creates default network isolation
func (r *OpenClawTenantReconciler) reconcileNetworkPolicy(ctx context.Context, tenant *openclawv1alpha1.OpenClawTenant) error {
	// Default deny all ingress/egress
	denyAll := &networkingv1.NetworkPolicy{}
	denyAll.Name = "default-deny-all"
	denyAll.Namespace = tenant.Name

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, denyAll, func() error {
		policyTypes := []networkingv1.PolicyType{}
		
		// Always deny ingress from outside
		denyAll.Spec.PodSelector = metav1.LabelSelector{}
		policyTypes = append(policyTypes, networkingv1.PolicyTypeIngress)
		
		if tenant.Spec.NetworkIsolation.DenyAllExternal {
			policyTypes = append(policyTypes, networkingv1.PolicyTypeEgress)
		}
		
		denyAll.Spec.PolicyTypes = policyTypes
		return nil
	})
	if err != nil {
		return err
	}

	// Allow DNS
	dnsPolicy := &networkingv1.NetworkPolicy{}
	dnsPolicy.Name = "allow-dns"
	dnsPolicy.Namespace = tenant.Name

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, dnsPolicy, func() error {
		dnsPolicy.Spec.PodSelector = metav1.LabelSelector{}
		dnsPolicy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}
		dnsPolicy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
			{
				To: []networkingv1.NetworkPolicyPeer{
					{
						NamespaceSelector: &metav1.LabelSelector{},
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"k8s-app": "kube-dns",
							},
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: &[]corev1.Protocol{corev1.ProtocolUDP}[0],
						Port: &intstr.IntOrString{IntVal: 53},
					},
				},
			},
		}
		return nil
	})

	return err
}

// countTenantInstances counts OpenClaw instances in the tenant namespace
func (r *OpenClawTenantReconciler) countTenantInstances(ctx context.Context, tenant *openclawv1alpha1.OpenClawTenant) (int32, error) {
	instances := &openclawv1alpha1.OpenClawList{}
	if err := r.List(ctx, instances, client.InNamespace(tenant.Name)); err != nil {
		return 0, err
	}
	return int32(len(instances.Items)), nil
}

// deleteTenantResources cleans up tenant resources
func (r *OpenClawTenantReconciler) deleteTenantResources(ctx context.Context, tenant *openclawv1alpha1.OpenClawTenant) error {
	// Check for remaining OpenClaw instances
	instanceCount, err := r.countTenantInstances(ctx, tenant)
	if err != nil {
		return err
	}
	if instanceCount > 0 {
		return fmt.Errorf("cannot delete tenant: %d OpenClaw instances still exist", instanceCount)
	}

	// Namespace will be deleted automatically via owner references/cascade
	return nil
}

// setTenantPhase updates tenant status phase
func (r *OpenClawTenantReconciler) setTenantPhase(
	ctx context.Context,
	tenant *openclawv1alpha1.OpenClawTenant,
	phase string,
	err error,
) (ctrl.Result, error) {
	tenant.Status.Phase = phase
	if err != nil {
		// Log error but don't fail permanently
		return ctrl.Result{}, err
	}
	if err := r.Status().Update(ctx, tenant); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller
func (r *OpenClawTenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawv1alpha1.OpenClawTenant{}).
		Complete(r)
}
