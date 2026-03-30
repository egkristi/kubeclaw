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
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openclawv1alpha1 "github.com/egkristi/kubeclaw/api/v1alpha1"
)

const (
	// Finalizer for cleanup
	openclawFinalizer = "openclaw.io/finalizer"

	// Phase constants
	PhasePending     = "Pending"
	PhaseRunning     = "Running"
	PhaseFailed      = "Failed"
	PhaseTerminating = "Terminating"

	// Condition types
	ConditionReady            = "Ready"
	ConditionWorkspaceSynced  = "WorkspaceSynced"
	ConditionNetworkPolicySet = "NetworkPolicySet"
	ConditionStorageReady     = "StorageReady"
)

// Config holds operator configuration
type Config struct {
	DefaultSandboxEnabled       bool
	DefaultSandboxLandlock      bool
	DefaultSandboxSeccomp       bool
	DefaultSandboxNetworkPolicy bool
	DefaultEgressMode           string
	OpenClawImage               string
	GitInitImage                string
}

// OpenClawReconciler reconciles a OpenClaw object
type OpenClawReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config Config
}

//+kubebuilder:rbac:groups=openclaw.io,resources=openclaws,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=openclaw.io,resources=openclaws/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=openclaw.io,resources=openclaws/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
func (r *OpenClawReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the OpenClaw instance
	openclaw := &openclawv1alpha1.OpenClaw{}
	if err := r.Get(ctx, req.NamespacedName, openclaw); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("OpenClaw resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get OpenClaw")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !openclaw.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, openclaw)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(openclaw, openclawFinalizer) {
		controllerutil.AddFinalizer(openclaw, openclawFinalizer)
		if err := r.Update(ctx, openclaw); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set initial phase
	if openclaw.Status.Phase == "" {
		openclaw.Status.Phase = PhasePending
		if err := r.Status().Update(ctx, openclaw); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile PVC if needed
	if err := r.reconcilePVC(ctx, openclaw); err != nil {
		logger.Error(err, "Failed to reconcile PVC")
		return r.setFailedStatus(ctx, openclaw, "PVCError", err.Error())
	}

	// Reconcile NetworkPolicy
	if err := r.reconcileNetworkPolicy(ctx, openclaw); err != nil {
		logger.Error(err, "Failed to reconcile NetworkPolicy")
		return r.setFailedStatus(ctx, openclaw, "NetworkPolicyError", err.Error())
	}

	// Reconcile Service
	if err := r.reconcileService(ctx, openclaw); err != nil {
		logger.Error(err, "Failed to reconcile Service")
		return r.setFailedStatus(ctx, openclaw, "ServiceError", err.Error())
	}

	// Reconcile Pod
	if err := r.reconcilePod(ctx, openclaw); err != nil {
		logger.Error(err, "Failed to reconcile Pod")
		return r.setFailedStatus(ctx, openclaw, "PodError", err.Error())
	}

	// Update status based on pod state
	return r.updateStatus(ctx, openclaw)
}

// reconcileDelete handles cleanup when OpenClaw is deleted
func (r *OpenClawReconciler) reconcileDelete(ctx context.Context, openclaw *openclawv1alpha1.OpenClaw) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling deletion of OpenClaw", "name", openclaw.Name)

	// Update phase to terminating
	openclaw.Status.Phase = PhaseTerminating
	if err := r.Status().Update(ctx, openclaw); err != nil {
		return ctrl.Result{}, err
	}

	// Remove finalizer to allow deletion
	controllerutil.RemoveFinalizer(openclaw, openclawFinalizer)
	if err := r.Update(ctx, openclaw); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcilePVC ensures PersistentVolumeClaim exists if storage type is PVC
func (r *OpenClawReconciler) reconcilePVC(ctx context.Context, openclaw *openclawv1alpha1.OpenClaw) error {
	logger := log.FromContext(ctx)

	// Skip if not using PVC
	if openclaw.Spec.Storage.Type != "PersistentVolumeClaim" {
		return nil
	}

	pvcName := fmt.Sprintf("%s-openclaw", openclaw.Name)
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: openclaw.Namespace}, pvc)

	if errors.IsNotFound(err) {
		logger.Info("Creating PVC", "name", pvcName)
		pvc = r.buildPVC(openclaw, pvcName)
		if err := controllerutil.SetControllerReference(openclaw, pvc, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, pvc)
	}

	if err != nil {
		return err
	}

	// Update status
	openclaw.Status.PVCName = pvcName
	openclaw.Status.StorageReady = pvc.Status.Phase == corev1.ClaimBound

	return nil
}

// buildPVC constructs a PersistentVolumeClaim for the OpenClaw instance
func (r *OpenClawReconciler) buildPVC(openclaw *openclawv1alpha1.OpenClaw, name string) *corev1.PersistentVolumeClaim {
	pvcSpec := openclaw.Spec.Storage.PersistentVolumeClaim

	size := "10Gi"
	if pvcSpec != nil && pvcSpec.Size != "" {
		size = pvcSpec.Size
	}

	accessMode := corev1.ReadWriteOnce
	if pvcSpec != nil && pvcSpec.AccessMode == "ReadWriteMany" {
		accessMode = corev1.ReadWriteMany
	} else if pvcSpec != nil && pvcSpec.AccessMode == "ReadOnlyMany" {
		accessMode = corev1.ReadOnlyMany
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: openclaw.Namespace,
			Labels:    r.labelsForOpenClaw(openclaw),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: mustParseQuantity(size),
				},
			},
		},
	}

	if pvcSpec != nil && pvcSpec.StorageClassName != nil {
		pvc.Spec.StorageClassName = pvcSpec.StorageClassName
	}

	return pvc
}

// reconcileNetworkPolicy ensures NetworkPolicy exists
func (r *OpenClawReconciler) reconcileNetworkPolicy(ctx context.Context, openclaw *openclawv1alpha1.OpenClaw) error {
	logger := log.FromContext(ctx)

	// Skip if network policy is disabled
	if !r.shouldCreateNetworkPolicy(openclaw) {
		return nil
	}

	npName := fmt.Sprintf("%s-egress", openclaw.Name)
	np := &networkingv1.NetworkPolicy{}
	err := r.Get(ctx, types.NamespacedName{Name: npName, Namespace: openclaw.Namespace}, np)

	if errors.IsNotFound(err) {
		logger.Info("Creating NetworkPolicy", "name", npName)
		np = r.buildNetworkPolicy(openclaw, npName)
		if err := controllerutil.SetControllerReference(openclaw, np, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, np)
	}

	if err != nil {
		return err
	}

	// Update status
	openclaw.Status.NetworkPolicyName = npName

	return nil
}

// shouldCreateNetworkPolicy determines if a NetworkPolicy should be created
func (r *OpenClawReconciler) shouldCreateNetworkPolicy(openclaw *openclawv1alpha1.OpenClaw) bool {
	// Check explicit spec setting
	if openclaw.Spec.Security.Sandbox.NetworkPolicy {
		return true
	}
	// Check default
	return r.Config.DefaultSandboxNetworkPolicy
}

// buildNetworkPolicy constructs a NetworkPolicy for egress control
func (r *OpenClawReconciler) buildNetworkPolicy(openclaw *openclawv1alpha1.OpenClaw, name string) *networkingv1.NetworkPolicy {
	labels := r.labelsForOpenClaw(openclaw)

	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: openclaw.Namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
		},
	}

	egressMode := openclaw.Spec.Security.Egress.Mode
	if egressMode == "" {
		egressMode = r.Config.DefaultEgressMode
	}

	switch egressMode {
	case "deny-all":
		// No egress rules = deny all
		np.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{}
	case "whitelist":
		// Allow DNS and specific domains
		np.Spec.Egress = r.buildWhitelistEgressRules(openclaw)
	default:
		// Default: allow all egress
		np.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{}}
	}

	return np
}

// buildWhitelistEgressRules creates egress rules for whitelisted domains
func (r *OpenClawReconciler) buildWhitelistEgressRules(openclaw *openclawv1alpha1.OpenClaw) []networkingv1.NetworkPolicyEgressRule {
	rules := []networkingv1.NetworkPolicyEgressRule{
		// Always allow DNS
		{
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: protocolPtr(corev1.ProtocolUDP),
					Port:     portPtr(53),
				},
				{
					Protocol: protocolPtr(corev1.ProtocolTCP),
					Port:     portPtr(53),
				},
			},
		},
		// Allow HTTPS egress (port 443)
		{
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: protocolPtr(corev1.ProtocolTCP),
					Port:     portPtr(443),
				},
			},
		},
	}

	return rules
}

// reconcileService ensures Service exists
func (r *OpenClawReconciler) reconcileService(ctx context.Context, openclaw *openclawv1alpha1.OpenClaw) error {
	logger := log.FromContext(ctx)

	svcName := fmt.Sprintf("%s-openclaw", openclaw.Name)
	svc := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: openclaw.Namespace}, svc)

	if errors.IsNotFound(err) {
		logger.Info("Creating Service", "name", svcName)
		svc = r.buildService(openclaw, svcName)
		if err := controllerutil.SetControllerReference(openclaw, svc, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, svc)
	}

	if err != nil {
		return err
	}

	// Update status
	openclaw.Status.ServiceName = svcName

	return nil
}

// buildService constructs a Service for the OpenClaw instance
func (r *OpenClawReconciler) buildService(openclaw *openclawv1alpha1.OpenClaw, name string) *corev1.Service {
	labels := r.labelsForOpenClaw(openclaw)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: openclaw.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       18789,
					TargetPort: intstr.FromInt(18789),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

// reconcilePod ensures the OpenClaw pod exists
func (r *OpenClawReconciler) reconcilePod(ctx context.Context, openclaw *openclawv1alpha1.OpenClaw) error {
	logger := log.FromContext(ctx)

	podName := fmt.Sprintf("%s-openclaw", openclaw.Name)
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: openclaw.Namespace}, pod)

	if errors.IsNotFound(err) {
		logger.Info("Creating Pod", "name", podName)
		pod = r.buildPod(openclaw, podName)
		if err := controllerutil.SetControllerReference(openclaw, pod, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, pod)
	}

	if err != nil {
		return err
	}

	// Update status with pod name
	openclaw.Status.PodName = podName

	return nil
}

// buildPod constructs the OpenClaw pod with init containers
func (r *OpenClawReconciler) buildPod(openclaw *openclawv1alpha1.OpenClaw, name string) *corev1.Pod {
	labels := r.labelsForOpenClaw(openclaw)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: openclaw.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			SecurityContext: r.buildPodSecurityContext(openclaw),
			InitContainers:  r.buildInitContainers(openclaw),
			Containers:      r.buildContainers(openclaw),
			Volumes:         r.buildVolumes(openclaw),
		},
	}

	return pod
}

// buildPodSecurityContext creates the pod-level security context
func (r *OpenClawReconciler) buildPodSecurityContext(openclaw *openclawv1alpha1.OpenClaw) *corev1.PodSecurityContext {
	runAsNonRoot := true
	runAsUser := int64(1000)
	fsGroup := int64(1000)

	psc := &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
		FSGroup:      &fsGroup,
	}

	// Add seccomp profile if enabled
	if r.shouldEnableSeccomp(openclaw) {
		psc.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
	}

	return psc
}

// shouldEnableSeccomp determines if seccomp should be enabled
func (r *OpenClawReconciler) shouldEnableSeccomp(openclaw *openclawv1alpha1.OpenClaw) bool {
	if openclaw.Spec.Security.Sandbox.Seccomp {
		return true
	}
	return r.Config.DefaultSandboxSeccomp
}

// buildInitContainers creates init containers for workspace setup
func (r *OpenClawReconciler) buildInitContainers(openclaw *openclawv1alpha1.OpenClaw) []corev1.Container {
	var initContainers []corev1.Container

	// Git clone init container (if workspace is configured)
	if openclaw.Spec.Workspace.Repository != "" {
		initContainers = append(initContainers, r.buildGitCloneInitContainer(openclaw))
	}

	return initContainers
}

// buildGitCloneInitContainer creates the git clone init container
func (r *OpenClawReconciler) buildGitCloneInitContainer(openclaw *openclawv1alpha1.OpenClaw) corev1.Container {
	branch := openclaw.Spec.Workspace.Branch
	if branch == "" {
		branch = "main"
	}

	container := corev1.Container{
		Name:    "git-clone",
		Image:   r.Config.GitInitImage,
		Command: []string{"sh", "-c"},
		Args: []string{
			fmt.Sprintf(`
set -e
if [ -d "/workspace/.git" ]; then
  echo "Workspace already exists, pulling latest changes"
  cd /workspace
  git fetch origin %s
  git reset --hard origin/%s
else
  echo "Cloning workspace repository"
  git clone --branch %s --depth 1 "${GIT_REPO}" /workspace
fi
echo "Workspace sync complete"
`, branch, branch, branch),
		},
		Env: []corev1.EnvVar{
			{
				Name:  "GIT_REPO",
				Value: openclaw.Spec.Workspace.Repository,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/workspace",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(false), // git needs to write
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}

	// Add credentials volume if specified
	if openclaw.Spec.Workspace.Credentials.Name != "" {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "git-credentials",
			MountPath: "/etc/git-credentials",
			ReadOnly:  true,
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "GIT_CREDENTIAL_FILE",
			Value: "/etc/git-credentials/token",
		})
	}

	return container
}

// buildContainers creates the main containers
func (r *OpenClawReconciler) buildContainers(openclaw *openclawv1alpha1.OpenClaw) []corev1.Container {
	container := corev1.Container{
		Name:  "openclaw",
		Image: r.Config.OpenClawImage,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: 18789,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: r.buildEnvVars(openclaw),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/home/openclaw/workspace",
			},
			{
				Name:      "openclaw-data",
				MountPath: "/home/openclaw/.openclaw",
			},
			{
				Name:      "tmp",
				MountPath: "/tmp",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(true),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(18789),
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(18789),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       5,
		},
	}

	// Apply resource requirements if specified
	if openclaw.Spec.Resources.Requests != nil || openclaw.Spec.Resources.Limits != nil {
		container.Resources = openclaw.Spec.Resources
	}

	return []corev1.Container{container}
}

// buildEnvVars creates environment variables for the OpenClaw container
func (r *OpenClawReconciler) buildEnvVars(openclaw *openclawv1alpha1.OpenClaw) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name:  "OPENCLAW_WORKSPACE",
			Value: "/home/openclaw/workspace",
		},
	}

	// Add model configuration
	if openclaw.Spec.Model.Provider != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "OPENCLAW_MODEL_PROVIDER",
			Value: openclaw.Spec.Model.Provider,
		})
	}

	if openclaw.Spec.Model.Model != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "OPENCLAW_MODEL",
			Value: openclaw.Spec.Model.Model,
		})
	}

	if openclaw.Spec.Model.BaseURL != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "OPENCLAW_MODEL_BASE_URL",
			Value: openclaw.Spec.Model.BaseURL,
		})
	}

	// Add API key from secret reference
	if openclaw.Spec.Model.APIKeySecretRef != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name: r.getAPIKeyEnvName(openclaw.Spec.Model.Provider),
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: openclaw.Spec.Model.APIKeySecretRef,
					},
					Key: "api-key",
				},
			},
		})
	}

	return envVars
}

// getAPIKeyEnvName returns the environment variable name for the provider's API key
func (r *OpenClawReconciler) getAPIKeyEnvName(provider string) string {
	switch provider {
	case "anthropic":
		return "ANTHROPIC_API_KEY"
	case "openai":
		return "OPENAI_API_KEY"
	default:
		return "API_KEY"
	}
}

// buildVolumes creates the volumes for the pod
func (r *OpenClawReconciler) buildVolumes(openclaw *openclawv1alpha1.OpenClaw) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	// Add openclaw-data volume based on storage type
	volumes = append(volumes, r.buildDataVolume(openclaw))

	// Add git credentials volume if needed
	if openclaw.Spec.Workspace.Credentials.Name != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "git-credentials",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: openclaw.Spec.Workspace.Credentials.Name,
				},
			},
		})
	}

	return volumes
}

// buildDataVolume creates the openclaw-data volume based on storage configuration
func (r *OpenClawReconciler) buildDataVolume(openclaw *openclawv1alpha1.OpenClaw) corev1.Volume {
	volume := corev1.Volume{
		Name: "openclaw-data",
	}

	switch openclaw.Spec.Storage.Type {
	case "PersistentVolumeClaim":
		volume.VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: fmt.Sprintf("%s-openclaw", openclaw.Name),
			},
		}
	case "EmptyDir":
		emptyDir := &corev1.EmptyDirVolumeSource{}
		if openclaw.Spec.Storage.EmptyDir != nil {
			if openclaw.Spec.Storage.EmptyDir.Medium == "Memory" {
				emptyDir.Medium = corev1.StorageMediumMemory
			}
			if openclaw.Spec.Storage.EmptyDir.SizeLimit != nil {
				emptyDir.SizeLimit = parseQuantityPtr(*openclaw.Spec.Storage.EmptyDir.SizeLimit)
			}
		}
		volume.VolumeSource = corev1.VolumeSource{
			EmptyDir: emptyDir,
		}
	default:
		// Default to EmptyDir
		volume.VolumeSource = corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	}

	return volume
}

// updateStatus updates the OpenClaw status based on pod state
func (r *OpenClawReconciler) updateStatus(ctx context.Context, openclaw *openclawv1alpha1.OpenClaw) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get the pod
	podName := fmt.Sprintf("%s-openclaw", openclaw.Name)
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: podName, Namespace: openclaw.Namespace}, pod)

	if errors.IsNotFound(err) {
		openclaw.Status.Phase = PhasePending
		openclaw.Status.PodName = ""
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		// Update phase based on pod status
		switch pod.Status.Phase {
		case corev1.PodRunning:
			openclaw.Status.Phase = PhaseRunning
		case corev1.PodPending:
			openclaw.Status.Phase = PhasePending
		case corev1.PodFailed:
			openclaw.Status.Phase = PhaseFailed
		default:
			openclaw.Status.Phase = PhasePending
		}
		openclaw.Status.PodName = podName

		// Check init container status for workspace sync
		for _, cs := range pod.Status.InitContainerStatuses {
			if cs.Name == "git-clone" {
				openclaw.Status.WorkspaceSynced = cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0
			}
		}
	}

	// Update conditions
	r.updateConditions(openclaw)

	if err := r.Status().Update(ctx, openclaw); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Requeue if not running
	if openclaw.Status.Phase != PhaseRunning {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// updateConditions updates the status conditions
func (r *OpenClawReconciler) updateConditions(openclaw *openclawv1alpha1.OpenClaw) {
	now := metav1.Now()

	// Ready condition
	readyCondition := openclawv1alpha1.Condition{
		Type:               ConditionReady,
		LastTransitionTime: now,
	}
	if openclaw.Status.Phase == PhaseRunning {
		readyCondition.Status = "True"
		readyCondition.Reason = "PodRunning"
		readyCondition.Message = "OpenClaw pod is running"
	} else {
		readyCondition.Status = "False"
		readyCondition.Reason = "PodNotReady"
		readyCondition.Message = fmt.Sprintf("OpenClaw pod is %s", openclaw.Status.Phase)
	}

	// WorkspaceSynced condition
	workspaceCondition := openclawv1alpha1.Condition{
		Type:               ConditionWorkspaceSynced,
		LastTransitionTime: now,
	}
	if openclaw.Status.WorkspaceSynced {
		workspaceCondition.Status = "True"
		workspaceCondition.Reason = "Synced"
		workspaceCondition.Message = "Workspace cloned successfully"
	} else {
		workspaceCondition.Status = "False"
		workspaceCondition.Reason = "NotSynced"
		workspaceCondition.Message = "Workspace not yet synced"
	}

	openclaw.Status.Conditions = []openclawv1alpha1.Condition{
		readyCondition,
		workspaceCondition,
	}
}

// setFailedStatus sets the status to failed with error details
func (r *OpenClawReconciler) setFailedStatus(ctx context.Context, openclaw *openclawv1alpha1.OpenClaw, reason, message string) (ctrl.Result, error) {
	openclaw.Status.Phase = PhaseFailed
	openclaw.Status.LastError = message

	if err := r.Status().Update(ctx, openclaw); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// labelsForOpenClaw returns the labels for selecting the resources
func (r *OpenClawReconciler) labelsForOpenClaw(openclaw *openclawv1alpha1.OpenClaw) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "openclaw",
		"app.kubernetes.io/instance":   openclaw.Name,
		"app.kubernetes.io/managed-by": "kubeclaw-operator",
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *OpenClawReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openclawv1alpha1.OpenClaw{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func protocolPtr(p corev1.Protocol) *corev1.Protocol {
	return &p
}

func portPtr(p int) *intstr.IntOrString {
	port := intstr.FromInt(p)
	return &port
}

func mustParseQuantity(s string) resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return resource.MustParse("10Gi")
	}
	return q
}

func parseQuantityPtr(s string) *resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		return nil
	}
	return &q
}
