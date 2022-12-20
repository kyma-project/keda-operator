/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/kyma-project/keda-manager/api/v1alpha1"
	"github.com/kyma-project/keda-manager/pkg/reconciler"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	chartNs = "kyma-system"
)

// KedaReconciler reconciles a Keda object
type KedaReconciler struct {
	reconciler.Reconciler
}

//+kubebuilder:rbac:groups="*",resources="*",verbs=get
//+kubebuilder:rbac:groups=external.metrics.k8s.io,resources="*",verbs="*"
//+kubebuilder:rbac:groups="",resources=configmaps;configmaps/status;events;services,verbs="*"
//+kubebuilder:rbac:groups="",resources=external;pods;secrets;serviceaccounts,verbs=list;watch;create;delete;update;patch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=create;delete
//+kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices,verbs=create;delete;update;patch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings;clusterroles;rolebindings,verbs=create;delete;update;patch
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=create;delete;update;patch
//+kubebuilder:rbac:groups="*",resources="*/scale",verbs="*"
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;watch;create;delete;update;patch
//+kubebuilder:rbac:groups=apps,resources=statefulsets;replicasets,verbs=list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs="*"
//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs="*"
//+kubebuilder:rbac:groups="keda.sh",resources=clustertriggerauthentications;clustertriggerauthentications/status;scaledjobs;scaledjobs/finalizers;scaledjobs/status;scaledobjects;scaledobjects/finalizers;scaledobjects/status;triggerauthentications;triggerauthentications/status,verbs="*"
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs="*"

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kedas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kedas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kedas/finalizers,verbs=update;patch

// SetupWithManager sets up the controller with the Manager.
func (r *KedaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Keda{}).
		Complete(r)
}

var ErrMultipleInstancesInNamespace = fmt.Errorf("namespace must not contain multiple module instances")

func GetLatest(ctx context.Context, c client.Client, nsName types.NamespacedName) (v1alpha1.Keda, error) {
	var instance v1alpha1.Keda
	err := c.Get(ctx, nsName, &instance)
	return instance, err
}

func buildStopAndError(err error) reconciler.ReconciliationAction {
	return func(ctx context.Context, c client.Client, req ctrl.Request) (bool, ctrl.Result, error) {
		instance, err := GetLatest(ctx, c, req.NamespacedName)
		if err != nil {
			return false, ctrl.Result{}, client.IgnoreNotFound(err)
		}

		instance.Status.State = "Error"
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               "Installed",
			Status:             "False",
			LastTransitionTime: metav1.Now(),
			Message:            "instance of a module already found on cluster",
			Reason:             "CrdCreationErr",
		})

		if err := c.Status().Update(ctx, &instance); err != nil {
			fmt.Println(err)
		}

		return false, ctrl.Result{
			RequeueAfter: time.Minute,
		}, err
	}
}

var (
	finalizer = "keda-manager.kyma-project.io/deletion-hook"

	defaultResult = ctrl.Result{
		Requeue: false,
	}

	kedaObjKey = client.ObjectKey{
		Name:      "keda",
		Namespace: "kyma-system",
	}

	main = reconciler.ReconciliationActions{}

	deletion = reconciler.ReconciliationActions{}
)
