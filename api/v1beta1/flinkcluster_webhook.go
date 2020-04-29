/*
Copyright 2019 Google LLC.

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
// +kubebuilder:docs-gen:collapse=Apache License

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// +kubebuilder:docs-gen:collapse=Go imports

var log = logf.Log.WithName("webhook")

// SetupWebhookWithManager adds webhook for FlinkCluster.
func (cluster *FlinkCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(cluster).
		Complete()
}

/*
Kubebuilder markers to generate webhook manifests.
This marker is responsible for generating a mutating webhook manifest.
The meaning of each marker can be found [here](/reference/markers/webhook.md).
*/

// +kubebuilder:webhook:path=/mutate-flinkoperator-k8s-io-v1beta1-flinkcluster,mutating=true,failurePolicy=fail,groups=flinkoperator.k8s.io,resources=flinkclusters,verbs=create;update,versions=v1beta1,name=mflinkcluster.flinkoperator.k8s.io

/*
We use the `webhook.Defaulter` interface to set defaults to our CRD.
A webhook will automatically be served that calls this defaulting.
The `Default` method is expected to mutate the receiver, setting the defaults.
*/

var _ webhook.Defaulter = &FlinkCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the
// type.
func (cluster *FlinkCluster) Default() {
	log.Info("default", "name", cluster.Name, "original", *cluster)
	if cluster.Spec.NativeSessionClusterJob != nil {
		log.Info("It's a NativeSessionCluster, will not set defaults.")
	} else {
		_SetDefault(cluster)
	}
	log.Info("default", "name", cluster.Name, "augmented", *cluster)
}

/*
This marker is responsible for generating a validating webhook manifest.
*/

// +kubebuilder:webhook:path=/validate-flinkoperator-k8s-io-v1beta1-flinkcluster,mutating=false,failurePolicy=fail,groups=flinkoperator.k8s.io,resources=flinkclusters,verbs=create;update,versions=v1beta1,name=vflinkcluster.flinkoperator.k8s.io

var _ webhook.Validator = &FlinkCluster{}
var validator = Validator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered
// for the type.
func (cluster *FlinkCluster) ValidateCreate() error {
	log.Info("Validate create", "name", cluster.Name)
	return validator.ValidateCreate(cluster)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered
// for the type.
func (cluster *FlinkCluster) ValidateUpdate(old runtime.Object) error {
	log.Info("Validate update", "name", cluster.Name)
	var oldCluster = old.(*FlinkCluster)
	return validator.ValidateUpdate(oldCluster, cluster)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered
// for the type.
func (cluster *FlinkCluster) ValidateDelete() error {
	log.Info("validate delete", "name", cluster.Name)

	// TODO
	return nil
}

// +kubebuilder:docs-gen:collapse=Validate object name
