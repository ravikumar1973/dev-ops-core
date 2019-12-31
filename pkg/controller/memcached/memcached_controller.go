package memcached

import (
	"context"

	cachev1alpha1 "github.com/example-inc/memcached-operator/pkg/apis/cache/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_memcached")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Memcached Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMemcached{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler *****
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("memcached-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Memcached
	err = c.Watch(&source.Kind{Type: &cachev1alpha1.Memcached{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Memcached
	//err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
	//	IsController: true,
	//	OwnerType:    &cachev1alpha1.Memcached{},
	//})
	//if err != nil {
	//	return err
	//}
	// Watch the ConfigMap actually residing inside the Cluster
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &cachev1alpha1.Memcached{},
	})
		if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileMemcached implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMemcached{}

// ReconcileMemcached reconciles a Memcached object
type ReconcileMemcached struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

//x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x-x

// Reconcile reads that state of the cluster for a Memcached object and makes changes based on the state read
// and what is in the Memcached.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMemcached) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Memcached")

	// Fetch the Memcached instance
	instance := &cachev1alpha1.Memcached{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Define new objects
	pod := newPodForCR(instance)
	cm := newCMForCR(instance)

	// Set Memcached cm as the owner and controller
	if err := controllerutil.SetControllerReference(instance, cm, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this ConfigMap already exists and app is also the same
	found := &corev1.ConfigMap{}
	pod_found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, found)
	//Check for Errors, if there is any or the deployment object not found (Check for CM only, as Pods spins up and die down)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating first time Pod + CM", "cm.Namespace", cm.Namespace, "cm.Name", cm.Name)
		err = r.client.Create(context.TODO(), cm)
		if err != nil {
			return reconcile.Result{}, err
		}
	
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}
	username := instance.Spec.Username //USE this for ConfigMaps
	if found.Data["Account"] != username {
		found.Data["Account"] = username
		err = r.client.Update(context.TODO(), found)
		if err != nil {
			reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
			return reconcile.Result{}, err
		}
	
	    err = r.client.Get(context.TODO(), types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, pod_found)
		if err != nil && errors.IsNotFound(err) {
			// Recreate the Pod reflecting changes in ConfigMaps
			err = r.client.Create(context.TODO(), pod)
			if err != nil {
			return reconcile.Result{}, err
			}
		}		// Spec updated - return and requeue
		return reconcile.Result{Requeue: true}, nil
	}
	// Pod already exists - don't requeue

	reqLogger.Info("Skip reconcile: cm already exists", "cm.Namespace", found.Namespace, "cm.Name", found.Name)
	return reconcile.Result{}, nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *cachev1alpha1.Memcached) *corev1.Pod {
	labels := labelsForMemcached(cr.Name)
	//naime:= cr.Spec.Username
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
					Env: []corev1.EnvVar{
						{
							Name: "USERNAME",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cr.Name,
									},
									Key: "Account",
								},	
							},
						},
					},	
				},
			},
		},
	}
}
// newConfigMap from the Parameters given in the CR Spec

func newCMForCR(cr *cachev1alpha1.Memcached) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
		},
		Data: 
			map[string]string{
			"Account": cr.Spec.Username,
			  // assume more attributes
			},
	}
}


//Generate the Lables required to be used for the Deployment Object
func labelsForMemcached(name string) map[string]string {
	return map[string]string{"app": "memcached", "memcached_cr": name}
}

