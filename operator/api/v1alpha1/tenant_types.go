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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenClawTenant defines a multi-tenant namespace for OpenClaw instances
type OpenClawTenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawTenantSpec   `json:"spec,omitempty"`
	Status OpenClawTenantStatus `json:"status,omitempty"`
}

// OpenClawTenantSpec defines the desired state of a tenant
type OpenClawTenantSpec struct {
	// Display name for the tenant
	DisplayName string `json:"displayName,omitempty"`

	// Description of the tenant's purpose
	Description string `json:"description,omitempty"`

	// Resource quotas for this tenant
	ResourceQuota ResourceQuotaSpec `json:"resourceQuota,omitempty"`

	// Limit ranges for this tenant
	LimitRange LimitRangeSpec `json:"limitRange,omitempty"`

	// Network isolation configuration
	NetworkIsolation NetworkIsolationSpec `json:"networkIsolation,omitempty"`

	// Allowed OpenClaw instance count
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MaxInstances *int32 `json:"maxInstances,omitempty"`

	// Owner reference (user or team)
	Owner string `json:"owner,omitempty"`

	// Labels to apply to all resources in this tenant
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to apply to all resources in this tenant
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ResourceQuotaSpec defines resource limits for the tenant
type ResourceQuotaSpec struct {
	// Total CPU limit across all instances in this tenant
	HardCPU resource.Quantity `json:"hardCPU,omitempty"`

	// Total memory limit across all instances
	HardMemory resource.Quantity `json:"hardMemory,omitempty"`

	// Total storage limit
	HardStorage resource.Quantity `json:"hardStorage,omitempty"`

	// Total number of PVCs allowed
	HardPVCs int32 `json:"hardPVCs,omitempty"`

	// Total number of services
	HardServices int32 `json:"hardServices,omitempty"`

	// Total number of pods
	HardPods int32 `json:"hardPods,omitempty"`
}

// LimitRangeSpec defines default resource constraints
type LimitRangeSpec struct {
	// Default CPU request for new instances
	DefaultCPURequest resource.Quantity `json:"defaultCPURequest,omitempty"`

	// Default CPU limit for new instances
	DefaultCPULimit resource.Quantity `json:"defaultCPULimit,omitempty"`

	// Default memory request
	DefaultMemoryRequest resource.Quantity `json:"defaultMemoryRequest,omitempty"`

	// Default memory limit
	DefaultMemoryLimit resource.Quantity `json:"defaultMemoryLimit,omitempty"`

	// Default storage request
	DefaultStorageRequest resource.Quantity `json:"defaultStorageRequest,omitempty"`

	// Min/max constraints
	MinCPU resource.Quantity `json:"minCPU,omitempty"`
	MaxCPU resource.Quantity `json:"maxCPU,omitempty"`
	MinMemory resource.Quantity `json:"minMemory,omitempty"`
	MaxMemory resource.Quantity `json:"maxMemory,omitempty"`
}

// NetworkIsolationSpec defines network policies for the tenant
type NetworkIsolationSpec struct {
	// Enable namespace-level network isolation
	Enabled bool `json:"enabled,omitempty"`

	// Allow ingress from these namespaces
	AllowedIngressNamespaces []string `json:"allowedIngressNamespaces,omitempty"`

	// Allow egress to these namespaces
	AllowedEgressNamespaces []string `json:"allowedEgressNamespaces,omitempty"`

	// Additional allowed external domains (beyond instance-level)
	AllowedExternalDomains []string `json:"allowedExternalDomains,omitempty"`

	// Deny all external egress (except allowedDomains)
	DenyAllExternal bool `json:"denyAllExternal,omitempty"`
}

// OpenClawTenantStatus defines the observed state of a tenant
type OpenClawTenantStatus struct {
	// Current phase: Pending, Active, Terminating, Failed
	Phase string `json:"phase,omitempty"`

	// Number of OpenClaw instances in this tenant
	InstanceCount int32 `json:"instanceCount,omitempty"`

	// Current resource usage
	UsedResources ResourceUsage `json:"usedResources,omitempty"`

	// Conditions for the tenant
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Creation timestamp
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

// ResourceUsage tracks actual resource consumption
type ResourceUsage struct {
	UsedCPU        resource.Quantity `json:"usedCPU,omitempty"`
	UsedMemory     resource.Quantity `json:"usedMemory,omitempty"`
	UsedStorage    resource.Quantity `json:"usedStorage,omitempty"`
	UsedPVCs       int32             `json:"usedPVCs,omitempty"`
	UsedPods       int32             `json:"usedPods,omitempty"`
	UsedServices   int32             `json:"usedServices,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster,shortName=oct
//+kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
//+kubebuilder:printcolumn:name="Instances",type=integer,JSONPath=`.status.instanceCount`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OpenClawTenantList contains a list of OpenClawTenant
type OpenClawTenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClawTenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenClawTenant{}, &OpenClawTenantList{})
}
