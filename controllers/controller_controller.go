/*
Copyright 2023.

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/controller-e2e-framework/test-1-controller/api/v1alpha1"
)

// ControllerReconciler reconciles a Controller object
type ControllerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=delivery.controller-e2e-framework,resources=controllers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=delivery.controller-e2e-framework,resources=controllers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Controller object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controller")

	obj := &v1alpha1.Controller{}
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to retrieve controller object: %w", err)
	}

	if obj.Spec.Ref == nil {
		logger.Info("no reference set for object, skipping")
		return ctrl.Result{}, nil
	}

	ref := obj.Spec.Ref

	// The contract says that the status of any Ref must contain a `Controller` field.
	u := &unstructured.Unstructured{}

	version, err := schema.ParseGroupVersion(ref.ApiVersion)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse group version: %w", err)
	}

	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   version.Group,
		Kind:    ref.Kind,
		Version: version.Version,
	})
	u.SetName(ref.Name)
	u.SetNamespace(obj.Namespace)

	if err := r.Get(ctx, types.NamespacedName{
		Namespace: obj.Namespace,
		Name:      u.GetName(),
	}, u); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find referenced object: %w", err)
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(u, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// update status
	status := u.Object["status"].(map[string]any)
	status["controller"] = true

	if err := patchHelper.Patch(ctx, u); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Controller{}).
		Complete(r)
}
