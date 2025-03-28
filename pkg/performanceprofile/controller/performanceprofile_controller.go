/*


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
	"os"
	"reflect"
	"time"

	"github.com/openshift/library-go/pkg/operator/configobserver/featuregates"

	apiconfigv1 "github.com/openshift/api/config/v1"
	apifeatures "github.com/openshift/api/features"
	mcov1 "github.com/openshift/api/machineconfiguration/v1"
	performancev2 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/performanceprofile/v2"
	tunedv1 "github.com/openshift/cluster-node-tuning-operator/pkg/apis/tuned/v1"
	ntoconfig "github.com/openshift/cluster-node-tuning-operator/pkg/config"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/components"
	profileutil "github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/components/profile"
	hypershiftconsts "github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/hypershift/consts"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/resources"
	"github.com/openshift/cluster-node-tuning-operator/pkg/performanceprofile/controller/performanceprofile/status"

	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serros "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	openshiftFinalizer  = "foreground-deletion"
	hypershiftFinalizer = "hypershift.openshift.io/foreground-deletion"
)

// PerformanceProfileReconciler reconciles a PerformanceProfile object
type PerformanceProfileReconciler struct {
	client.Client
	ManagementClient  client.Client
	Recorder          record.EventRecorder
	FeatureGate       featuregates.FeatureGate
	ComponentsHandler components.Handler
	StatusWriter      status.Writer
}

// SetupWithManager creates a new PerformanceProfile Controller and adds it to the Manager.
// The Manager will set fields on the Controller and Start it when the Manager is Started.
func (r *PerformanceProfileReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// we want to initate reconcile loop only on change under labels or spec of the object
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !validateUpdateEvent(e.ObjectOld, e.ObjectNew) {
				return false
			}

			return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() ||
				!apiequality.Semantic.DeepEqual(e.ObjectNew.GetLabels(), e.ObjectOld.GetLabels())
		},
	}

	kubeletPredicates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if validateUpdateEvent(e.ObjectOld, e.ObjectNew) {
				return false
			}

			kubeletOld := e.ObjectOld.(*mcov1.KubeletConfig)
			kubeletNew := e.ObjectNew.(*mcov1.KubeletConfig)

			return kubeletOld.GetGeneration() != kubeletNew.GetGeneration() ||
				!reflect.DeepEqual(kubeletOld.Status.Conditions, kubeletNew.Status.Conditions)
		},
	}

	mcpPredicates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !validateUpdateEvent(e.ObjectOld, e.ObjectNew) {
				return false
			}

			mcpOld := e.ObjectOld.(*mcov1.MachineConfigPool)
			mcpNew := e.ObjectNew.(*mcov1.MachineConfigPool)

			return !reflect.DeepEqual(mcpOld.Status.Conditions, mcpNew.Status.Conditions)
		},
	}

	tunedProfilePredicates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !validateUpdateEvent(e.ObjectOld, e.ObjectNew) {
				return false
			}

			tunedProfileOld := e.ObjectOld.(*tunedv1.Profile)
			tunedProfileNew := e.ObjectNew.(*tunedv1.Profile)

			return !reflect.DeepEqual(tunedProfileOld.Status.Conditions, tunedProfileNew.Status.Conditions)
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&performancev2.PerformanceProfile{}).
		Owns(&mcov1.MachineConfig{}, builder.WithPredicates(p)).
		Owns(&mcov1.KubeletConfig{}, builder.WithPredicates(kubeletPredicates)).
		Owns(&tunedv1.Tuned{}, builder.WithPredicates(p)).
		Owns(&nodev1.RuntimeClass{}, builder.WithPredicates(p)).
		Watches(&mcov1.MachineConfigPool{},
			handler.EnqueueRequestsFromMapFunc(r.mcpToPerformanceProfile),
			builder.WithPredicates(mcpPredicates)).
		Watches(&tunedv1.Profile{},
			handler.EnqueueRequestsFromMapFunc(r.tunedProfileToPerformanceProfile),
			builder.WithPredicates(tunedProfilePredicates),
		).
		Complete(r)
}

func (r *PerformanceProfileReconciler) SetupWithManagerForHypershift(mgr ctrl.Manager, managementCluster cluster.Cluster) error {
	// Running on HyperShift controller should watch for the ConfigMaps created by HyperShift Operator in the
	// controller namespace with the right label.
	performanceProfileConfigMapPredicate := predicate.TypedFuncs[*corev1.ConfigMap]{
		UpdateFunc: func(ue event.TypedUpdateEvent[*corev1.ConfigMap]) bool {
			if !validateUpdateEvent(ue.ObjectOld, ue.ObjectNew) {
				klog.V(4).InfoS("UpdateEvent not valid", "objectName", ue.ObjectOld.GetName())
				return false
			}
			return validateLabels(ue.ObjectNew, hypershiftconsts.ControllerGeneratedPerformanceProfileConfigMapLabel, "UpdateEvent")
		},
		CreateFunc: func(ce event.TypedCreateEvent[*corev1.ConfigMap]) bool {
			if ce.Object == nil {
				klog.Error("Create event has no runtime object")
				return false
			}
			return validateLabels(ce.Object, hypershiftconsts.ControllerGeneratedPerformanceProfileConfigMapLabel, "CreateEvent")
		},
		DeleteFunc: func(de event.TypedDeleteEvent[*corev1.ConfigMap]) bool {
			if de.Object == nil {
				klog.Error("Delete event has no runtime object")
				return false
			}
			return validateLabels(de.Object, hypershiftconsts.ControllerGeneratedPerformanceProfileConfigMapLabel, "DeleteEvent")
		},
	}
	OwnedObjectsConfigMapsPredicates := predicate.TypedFuncs[*corev1.ConfigMap]{
		UpdateFunc: func(ue event.TypedUpdateEvent[*corev1.ConfigMap]) bool {
			if !validateUpdateEvent(ue.ObjectOld, ue.ObjectNew) {
				klog.V(4).InfoS("UpdateEvent not valid", "objectName", ue.ObjectOld.GetName())
				return false
			}
			return validateLabels(ue.ObjectNew, hypershiftconsts.PerformanceProfileNameLabel, "UpdateEvent")
		},
		CreateFunc: func(ce event.TypedCreateEvent[*corev1.ConfigMap]) bool {
			if ce.Object == nil {
				klog.Error("Create event has no runtime object")
				return false
			}
			return validateLabels(ce.Object, hypershiftconsts.PerformanceProfileNameLabel, "CreateEvent")
		},
		DeleteFunc: func(de event.TypedDeleteEvent[*corev1.ConfigMap]) bool {
			if de.Object == nil {
				klog.Error("Delete event has no runtime object")
				return false
			}
			return validateLabels(de.Object, hypershiftconsts.PerformanceProfileNameLabel, "DeleteEvent")
		},
	}

	rtClassPredicate := predicate.TypedFuncs[*nodev1.RuntimeClass]{
		UpdateFunc: func(e event.TypedUpdateEvent[*nodev1.RuntimeClass]) bool {
			if !validateUpdateEvent(e.ObjectOld, e.ObjectNew) {
				return false
			}
			return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() ||
				!apiequality.Semantic.DeepEqual(e.ObjectNew.GetLabels(), e.ObjectOld.GetLabels())
		},
	}

	tunedProfilePredicates := predicate.TypedFuncs[*tunedv1.Profile]{
		UpdateFunc: func(e event.TypedUpdateEvent[*tunedv1.Profile]) bool {
			if !validateUpdateEvent(e.ObjectOld, e.ObjectNew) {
				return false
			}

			tunedProfileOld := e.ObjectOld
			tunedProfileNew := e.ObjectNew

			return !reflect.DeepEqual(tunedProfileOld.Status.Conditions, tunedProfileNew.Status.Conditions)
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("performanceprofile_controller").
		// we can't use For() and Owns(), because this calls are using the cache of the hosted cluster's client.
		// instead, we explicitly set a watch using the cache of the management cluster.
		WatchesRawSource(source.Kind(managementCluster.GetCache(),
			&corev1.ConfigMap{},
			&handler.TypedEnqueueRequestForObject[*corev1.ConfigMap]{},
			performanceProfileConfigMapPredicate)).
		WatchesRawSource(source.Kind(managementCluster.GetCache(),
			&corev1.ConfigMap{},
			handler.TypedEnqueueRequestForOwner[*corev1.ConfigMap](r.ManagementClient.Scheme(), r.ManagementClient.RESTMapper(), &corev1.ConfigMap{}, handler.OnlyControllerOwner()),
			OwnedObjectsConfigMapsPredicates)).
		// the controller watches over the RuntimeClass in the hosted cluster
		WatchesRawSource(source.Kind(mgr.GetCache(),
			&nodev1.RuntimeClass{},
			handler.TypedEnqueueRequestForOwner[*nodev1.RuntimeClass](mgr.GetScheme(), mgr.GetRESTMapper(), &corev1.ConfigMap{}, handler.OnlyControllerOwner()),
			rtClassPredicate)).
		WatchesRawSource(source.Kind(mgr.GetCache(),
			&tunedv1.Profile{},
			handler.TypedEnqueueRequestsFromMapFunc[*tunedv1.Profile](r.tunedProfileToPerformanceProfileForHypershift),
			tunedProfilePredicates)).
		Complete(r)
}

func validateLabels(obj client.Object, label, eventType string) bool {
	if _, ok := obj.GetLabels()[label]; ok {
		klog.V(4).InfoS("Label found", "label", label, "eventType", eventType, "objectName", obj.GetName())
		return true
	}
	return false
}

func (r *PerformanceProfileReconciler) tunedProfileToPerformanceProfileForHypershift(ctx context.Context, tunedProfileObj *tunedv1.Profile) []reconcile.Request {
	node := &corev1.Node{}
	key := types.NamespacedName{
		// the tuned profile name is the same as node
		Name: tunedProfileObj.GetName(),
	}

	if err := r.Get(ctx, key, node); err != nil {
		klog.Errorf("failed to get the tuned profile %+v: %v", key, err)
		return nil
	}

	npName, ok := node.Labels[hypershiftconsts.NodePoolNameLabel]
	if !ok {
		klog.Errorf("node label %s not found in node %s", hypershiftconsts.NodePoolNameLabel, node.GetName())
	}
	cmList := &corev1.ConfigMapList{}
	err := r.ManagementClient.List(ctx, cmList, client.MatchingLabels{hypershiftconsts.NodePoolNameLabel: npName, hypershiftconsts.ControllerGeneratedPerformanceProfileConfigMapLabel: "true"})
	if err != nil {
		klog.Errorf("failed to get performance profiles ConfigMaps: %v", err)
		return nil
	}
	if len(cmList.Items) == 0 {
		klog.V(4).InfoS("no performance profile ConfigMap found that matches label", "label", hypershiftconsts.NodePoolNameLabel)
		return nil
	}
	if len(cmList.Items) > 1 {
		klog.Errorf("no more than a single performance profile ConfigMap that matches label %s is expected to be found", hypershiftconsts.NodePoolNameLabel)
		return nil
	}
	return []reconcile.Request{{NamespacedName: namespacedName(&cmList.Items[0])}}
}

func (r *PerformanceProfileReconciler) mcpToPerformanceProfile(ctx context.Context, mcpObj client.Object) []reconcile.Request {
	mcp := &mcov1.MachineConfigPool{}

	key := types.NamespacedName{
		Namespace: mcpObj.GetNamespace(),
		Name:      mcpObj.GetName(),
	}
	if err := r.Get(ctx, key, mcp); err != nil {
		klog.Errorf("failed to get the machine config pool %+v: %v", key, err)
		return nil
	}

	profiles := &performancev2.PerformanceProfileList{}
	if err := r.List(context.TODO(), profiles); err != nil {
		klog.Errorf("failed to get performance profiles: %v", err)
		return nil
	}

	return mcpToPerformanceProfileReconcileRequests(profiles, mcp)
}

func mcpToPerformanceProfileReconcileRequests(profiles *performancev2.PerformanceProfileList, mcp *mcov1.MachineConfigPool) []reconcile.Request {
	var requests []reconcile.Request
	for i, profile := range profiles.Items {
		profileNodeSelector := labels.Set(profile.Spec.NodeSelector)
		mcpNodeSelector, err := metav1.LabelSelectorAsSelector(mcp.Spec.NodeSelector)
		if err != nil {
			klog.Errorf("failed to parse the selector %v: %v", mcp.Spec.NodeSelector, err)
			return nil
		}

		if mcpNodeSelector.Matches(profileNodeSelector) {
			requests = append(requests, reconcile.Request{NamespacedName: namespacedName(&profiles.Items[i])})
		}
	}

	return requests
}

func (r *PerformanceProfileReconciler) tunedProfileToPerformanceProfile(ctx context.Context, tunedProfileObj client.Object) []reconcile.Request {
	node := &corev1.Node{}
	key := types.NamespacedName{
		// the tuned profile name is the same as node
		Name: tunedProfileObj.GetName(),
	}

	if err := r.Get(ctx, key, node); err != nil {
		klog.Errorf("failed to get the tuned profile %+v: %v", key, err)
		return nil
	}

	profiles := &performancev2.PerformanceProfileList{}
	if err := r.List(context.TODO(), profiles); err != nil {
		klog.Errorf("failed to get performance profiles: %v", err)
		return nil
	}

	var requests []reconcile.Request
	for i, profile := range profiles.Items {
		profileNodeSelector := labels.Set(profile.Spec.NodeSelector)
		nodeLabels := labels.Set(node.Labels)
		if profileNodeSelector.AsSelector().Matches(nodeLabels) {
			requests = append(requests, reconcile.Request{NamespacedName: namespacedName(&profiles.Items[i])})
		}
	}

	return requests
}

func getInfraPartitioningMode(ctx context.Context, client client.Client) (pinning apiconfigv1.CPUPartitioningMode, err error) {
	key := types.NamespacedName{
		Name: "cluster",
	}
	infra := &apiconfigv1.Infrastructure{}

	if err = client.Get(ctx, key, infra); err != nil {
		return
	}

	return infra.Status.CPUPartitioning, nil
}

func validateUpdateEvent(old, new metav1.Object) bool {
	if old == nil {
		klog.Error("Update event has no old runtime object to update")
		return false
	}
	if new == nil {
		klog.Error("Update event has no new runtime object for update")
		return false
	}

	return true
}

// +kubebuilder:rbac:groups="",resources=events,verbs=*
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=performance.openshift.io,resources=performanceprofiles;performanceprofiles/status;performanceprofiles/finalizers,verbs=*
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigs;kubeletconfigs,verbs=*
// +kubebuilder:rbac:groups=machineconfiguration.openshift.io,resources=machineconfigpools,verbs=get;list;watch
// +kubebuilder:rbac:groups=tuned.openshift.io,resources=tuneds;profiles,verbs=*
// +kubebuilder:rbac:groups=node.k8s.io,resources=runtimeclasses,verbs=*
// +kubebuilder:rbac:groups=config.openshift.io,resources=infrastructures,verbs=get;list;watch
// +kubebuilder:rbac:namespace="openshift-cluster-node-tuning-operator",groups=core,resources=pods;services;services/finalizers;configmaps,verbs=*
// +kubebuilder:rbac:namespace="openshift-cluster-node-tuning-operator",groups=coordination.k8s.io,resources=leases,verbs=create;get;list;update
// +kubebuilder:rbac:namespace="openshift-cluster-node-tuning-operator",groups=apps,resourceNames=performance-operator,resources=deployments/finalizers,verbs=update
// +kubebuilder:rbac:namespace="openshift-cluster-node-tuning-operator",groups=monitoring.coreos.com,resources=servicemonitors,verbs=*

// Reconcile reads that state of the cluster for a PerformanceProfile object and makes changes based on the state read
// and what is in the PerformanceProfile.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *PerformanceProfileReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	co, err := resources.GetClusterOperator(ctx, r.Client)
	if err != nil {
		klog.Errorf("failed to get ClusterOperator: %v", err)
		return reconcile.Result{}, err
	}

	operatorReleaseVersion := os.Getenv("RELEASE_VERSION")
	operandReleaseVersion := operatorv1helpers.FindOperandVersion(co.Status.Versions, tunedv1.TunedOperandName)
	if operandReleaseVersion == nil || operatorReleaseVersion != operandReleaseVersion.Version {
		// Upgrade in progress. Should happen rarely, so we omit V()
		klog.Infof("operator and operand release versions do not match")
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	klog.V(4).InfoS("Reconciling", "reqNamespace", req.NamespacedName)
	defer klog.V(4).InfoS("Exit Reconciling", "reqNamespace", req.NamespacedName)

	var instance client.Object
	instance = &performancev2.PerformanceProfile{}
	finalizer := openshiftFinalizer
	if ntoconfig.InHyperShift() {
		instance = &corev1.ConfigMap{}
		finalizer = hypershiftFinalizer
	}

	err = r.ManagementClient.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8serros.IsNotFound(err) {
			// Request object isn't found, could have been deleted after reconciled request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic, use finalizers.
			// Return and don't requeue
			klog.InfoS("Instance not found", "reqNamespace", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		klog.ErrorS(err, "Reading failure", "reqNamespace", req.NamespacedName)
		return reconcile.Result{}, err
	}
	instanceKey := client.ObjectKeyFromObject(instance)
	if instance.GetDeletionTimestamp() != nil {
		// delete components
		if err := r.ComponentsHandler.Delete(ctx, instance.GetName()); err != nil {
			klog.Errorf("failed to delete components: %v", err)
			r.Recorder.Eventf(instance, corev1.EventTypeWarning, "Deletion failed", "Failed to delete components: %v", err)
			return reconcile.Result{}, err
		}
		r.Recorder.Eventf(instance, corev1.EventTypeNormal, "Deletion succeeded", "Succeeded to delete all components")

		if r.ComponentsHandler.Exists(ctx, instance.GetName()) {
			return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		}

		// remove finalizer
		if hasFinalizer(instance, finalizer) {
			klog.InfoS("Remove finalizer", "instance", instanceKey.String())
			removeFinalizer(instance, finalizer)
			if err := r.ManagementClient.Update(ctx, instance); err != nil {
				return reconcile.Result{}, err
			}
			klog.InfoS("Finalizer Removed", "instance", instanceKey.String())
			return reconcile.Result{}, nil
		}
	}

	// add finalizer
	if !hasFinalizer(instance, finalizer) {
		klog.InfoS("Add finalizer", "instance", instanceKey.String())
		instance.SetFinalizers(append(instance.GetFinalizers(), finalizer))
		if prof, ok := instance.(*performancev2.PerformanceProfile); ok {
			prof.Status.Conditions = status.GetProgressingConditions("DeploymentStarting", "Deployment is starting")
		}
		if err = r.ManagementClient.Update(ctx, instance); err != nil {
			return reconcile.Result{}, err
		}
		klog.InfoS("Finalizer added", "instance", instanceKey.String())
		// we exit reconcile loop because we will have additional update reconcile
		return reconcile.Result{}, nil
	}

	pinningMode, err := getInfraPartitioningMode(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	profileMCP, result, err := r.getAndValidateMCP(ctx, instance)
	if result != nil {
		return *result, err
	}

	// apply components
	err = r.ComponentsHandler.Apply(ctx, instance, r.Recorder, &components.Options{
		ProfileMCP: profileMCP,
		MachineConfig: components.MachineConfigOptions{
			PinningMode: &pinningMode,
		},
		MixedCPUsFeatureGateEnabled: r.isMixedCPUsFeatureGateEnabled(),
	})
	if err != nil {
		klog.Errorf("failed to deploy performance profile %q components: %v", instance.GetName(), err)
		r.Recorder.Eventf(instance, corev1.EventTypeWarning, "Creation failed", "Failed to create all components: %v", err)
		conditions := status.GetDegradedConditions(status.ConditionReasonComponentsCreationFailed, err.Error())
		if err := r.StatusWriter.Update(ctx, instance, conditions); err != nil {
			klog.Errorf("failed to update performance profile %q status: %v", instance.GetName(), err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, err
	}
	err = r.StatusWriter.UpdateOwnedConditions(ctx, instance)
	if err != nil {
		klog.Errorf("failed to update performance profile %q status: %v", instance.GetName(), err)
	}
	return ctrl.Result{}, nil
}

func (r *PerformanceProfileReconciler) isMixedCPUsFeatureGateEnabled() bool {
	return r.FeatureGate.Enabled(apifeatures.FeatureGateMixedCPUsAllocation)
}

func hasFinalizer(obj client.Object, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

func (r *PerformanceProfileReconciler) getAndValidateMCP(ctx context.Context, instance client.Object) (*mcov1.MachineConfigPool, *reconcile.Result, error) {
	profile, ok := instance.(*performancev2.PerformanceProfile)
	// can happen on HyperShift, which expects ConfigMap instead.
	// but on hypershift we do not have MCPs anyway, so it's fine to return empty here.
	if !ok {
		return nil, nil, nil
	}
	profileMCP, err := resources.GetMachineConfigPoolByProfile(ctx, r.Client, profile)
	if err != nil {
		conditions := status.GetDegradedConditions(status.ConditionFailedToFindMachineConfigPool, err.Error())
		if err := r.StatusWriter.Update(ctx, profile, conditions); err != nil {
			klog.Errorf("failed to update performance profile %q status: %v", profile.GetName(), err)
			return nil, &reconcile.Result{}, err
		}
		return nil, &reconcile.Result{}, nil
	}

	if err := validateProfileMachineConfigPool(profile, profileMCP); err != nil {
		conditions := status.GetDegradedConditions(status.ConditionBadMachineConfigLabels, err.Error())
		if err := r.StatusWriter.Update(ctx, profile, conditions); err != nil {
			klog.Errorf("failed to update performance profile %q status: %v", profile.GetName(), err)
			return nil, &reconcile.Result{}, err
		}
		return nil, &reconcile.Result{}, nil
	}
	return profileMCP, nil, nil
}

func removeFinalizer(obj client.Object, finalizer string) {
	var finalizers []string
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			continue
		}
		finalizers = append(finalizers, f)
	}
	obj.SetFinalizers(finalizers)
}

func namespacedName(obj metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

func validateProfileMachineConfigPool(profile *performancev2.PerformanceProfile, profileMCP *mcov1.MachineConfigPool) error {
	if profileMCP.Spec.MachineConfigSelector.Size() == 0 {
		return fmt.Errorf("the MachineConfigPool %q machineConfigSelector is nil", profileMCP.Name)
	}

	if len(profileMCP.Labels) == 0 {
		return fmt.Errorf("the MachineConfigPool %q does not have any labels that can be used to bind it together with KubeletConfing", profileMCP.Name)
	}

	// we can not guarantee that our generated label for the machine config selector will be the right one
	// but at least we can validate that the MCP will consume our machine config
	machineConfigLabels := profileutil.GetMachineConfigLabel(profile)
	mcpMachineConfigSelector, err := metav1.LabelSelectorAsSelector(profileMCP.Spec.MachineConfigSelector)
	if err != nil {
		return err
	}

	if !mcpMachineConfigSelector.Matches(labels.Set(machineConfigLabels)) {
		if len(profile.Spec.MachineConfigLabel) > 0 {
			return fmt.Errorf("the machine config labels %v provided via profile.spec.machineConfigLabel do not match the MachineConfigPool %q machineConfigSelector %q", machineConfigLabels, profileMCP.Name, mcpMachineConfigSelector.String())
		}

		return fmt.Errorf("the machine config labels %v generated from the profile.spec.nodeSelector %v do not match the MachineConfigPool %q machineConfigSelector %q", machineConfigLabels, profile.Spec.NodeSelector, profileMCP.Name, mcpMachineConfigSelector.String())
	}

	return nil
}
