/*
Copyright 2026 Erling Kristiansen.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,


























)	AddToScheme = SchemeBuilder.AddToScheme	// AddToScheme adds the types in this group-version to the given scheme.	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}	// SchemeBuilder is used to add go types to the GroupVersionKind scheme	GroupVersion = schema.GroupVersion{Group: "openclaw.io", Version: "v1alpha1"}	// GroupVersion is group version used to register these objectsvar ()	"sigs.k8s.io/controller-runtime/pkg/scheme"	"k8s.io/apimachinery/pkg/runtime/schema"import (package v1alpha1// +groupName=openclaw.io// +kubebuilder:object:generate=true// Package v1alpha1 contains API Schema definitions for the openclaw v1alpha1 API group*/limitations under the License.See the License for the specific language governing permissions andWITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1
/*
Copyright 2026 Erling Kristiansen.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,