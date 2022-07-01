/*
Copyright 2021.

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	nsoptions "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/namespaces"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
)

var clog = ctrl.Log.WithName("controller_v1beta1activemqartemis")

var namespaceToConfigHandler = make(map[types.NamespacedName]common.ActiveMQArtemisConfigHandler)

func GetBrokerConfigHandler(brokerNamespacedName types.NamespacedName) (handler common.ActiveMQArtemisConfigHandler) {
	for _, handler := range namespaceToConfigHandler {
		if handler.IsApplicableFor(brokerNamespacedName) {
			return handler
		}
	}
	return nil
}

func (r *ActiveMQArtemisReconciler) UpdatePodForSecurity(securityHandlerNamespacedName types.NamespacedName, handler common.ActiveMQArtemisConfigHandler) error {

	existingCrs := &brokerv1beta1.ActiveMQArtemisList{}
	var err error
	opts := &client.ListOptions{}
	if err = r.Client.List(context.TODO(), existingCrs, opts); err == nil {
		var candidate types.NamespacedName
		for index, artemis := range existingCrs.Items {
			candidate.Name = artemis.Name
			candidate.Namespace = artemis.Namespace
			if handler.IsApplicableFor(candidate) {
				clog.Info("force reconcile for security", "handler", securityHandlerNamespacedName, "CR", candidate)
				r.events <- event.GenericEvent{Object: &existingCrs.Items[index]}
			}
		}
	}
	return err
}

func (r *ActiveMQArtemisReconciler) RemoveBrokerConfigHandler(namespacedName types.NamespacedName) {
	clog.Info("Removing config handler", "name", namespacedName)
	oldHandler, ok := namespaceToConfigHandler[namespacedName]
	if ok {
		delete(namespaceToConfigHandler, namespacedName)
		clog.Info("Handler removed", "name", namespacedName)
		r.UpdatePodForSecurity(namespacedName, oldHandler)
	}
}

func (r *ActiveMQArtemisReconciler) AddBrokerConfigHandler(namespacedName types.NamespacedName, handler common.ActiveMQArtemisConfigHandler, toReconcile bool) error {
	if _, ok := namespaceToConfigHandler[namespacedName]; ok {
		clog.V(1).Info("There is an old config handler, it'll be replaced")
	}
	namespaceToConfigHandler[namespacedName] = handler
	clog.V(1).Info("A new config handler has been added", "handler", handler)
	if toReconcile {
		clog.V(1).Info("Updating broker security")
		return r.UpdatePodForSecurity(namespacedName, handler)
	}
	return nil
}

// ActiveMQArtemisReconciler reconciles a ActiveMQArtemis object
type ActiveMQArtemisReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	WatchOptions nsoptions.WatchOptions
	events       chan event.GenericEvent
}

//run 'make manifests' after changing the following rbac markers

//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemises,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemises/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemises/finalizers,verbs=update
//+kubebuilder:rbac:groups=broker.amq.io,resources=pods,verbs=get;list
//+kubebuilder:rbac:groups="",resources=pods;services;endpoints;persistentvolumeclaims;events;configmaps;secrets;routes;serviceaccounts,verbs=*
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=*
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes;routes/custom-host;routes/status,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=create;get;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ActiveMQArtemis object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ActiveMQArtemisReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	reqLogger := ctrl.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name, "Reconciling", "ActiveMQArtemis")

	var err error = nil

	if r.WatchOptions.Match(request.Namespace) {

		// we are responsible

		customResource := &brokerv1beta1.ActiveMQArtemis{}

		// Fetch the ActiveMQArtemis instance
		// When first creating this will have err == nil
		// When deleting after creation this will have err NotFound
		// When deleting before creation reconcile won't be called
		if err = r.Get(context.TODO(), request.NamespacedName, customResource); err == nil {

			namer := MakeNamers(customResource)
			reconciler := ActiveMQArtemisReconcilerImpl{}

			reconciler.Process(customResource, *namer, r.Client, r.Scheme)

			UpdatePodStatus(customResource, r.Client, request.NamespacedName)

		} else if errors.IsNotFound(err) {
			reqLogger.Info("ActiveMQArtemis Controller Reconcile encountered a IsNotFound, for request NamespacedName " + request.NamespacedName.String())
			err = nil
		}
	} else {
		reqLogger.Info("Request not in watch list, ignore", "request", request)
	}

	if err == nil {
		return ctrl.Result{}, err
	} else {
		reqLogger.Error(err, "ActiveMQArtemis Controller Reconcile, requeuing request", "Request Namespace", request.Namespace, "Request Name", request.Name)
		return ctrl.Result{RequeueAfter: common.GetReconcileResyncPeriod()}, err
	}
}

type Namers struct {
	SsGlobalName                  string
	SsNameBuilder                 namer.NamerData
	SvcHeadlessNameBuilder        namer.NamerData
	SvcPingNameBuilder            namer.NamerData
	PodsNameBuilder               namer.NamerData
	SecretsCredentialsNameBuilder namer.NamerData
	SecretsConsoleNameBuilder     namer.NamerData
	SecretsNettyNameBuilder       namer.NamerData
	LabelBuilder                  selectors.LabelerData
	GLOBAL_DATA_PATH              string
}

func MakeNamers(customResource *brokerv1beta1.ActiveMQArtemis) *Namers {
	newNamers := Namers{
		SsGlobalName:                  "",
		SsNameBuilder:                 namer.NamerData{},
		SvcHeadlessNameBuilder:        namer.NamerData{},
		SvcPingNameBuilder:            namer.NamerData{},
		PodsNameBuilder:               namer.NamerData{},
		SecretsCredentialsNameBuilder: namer.NamerData{},
		SecretsConsoleNameBuilder:     namer.NamerData{},
		SecretsNettyNameBuilder:       namer.NamerData{},
		LabelBuilder:                  selectors.LabelerData{},
		GLOBAL_DATA_PATH:              "/opt/" + customResource.Name + "/data",
	}
	newNamers.SsNameBuilder.Base(customResource.Name).Suffix("ss").Generate()
	newNamers.SsGlobalName = customResource.Name
	newNamers.SvcHeadlessNameBuilder.Prefix(customResource.Name).Base("hdls").Suffix("svc").Generate()
	newNamers.SvcPingNameBuilder.Prefix(customResource.Name).Base("ping").Suffix("svc").Generate()
	newNamers.PodsNameBuilder.Base(customResource.Name).Suffix("container").Generate()
	newNamers.SecretsCredentialsNameBuilder.Prefix(customResource.Name).Base("credentials").Suffix("secret").Generate()
	newNamers.SecretsConsoleNameBuilder.Prefix(customResource.Name).Base("console").Suffix("secret").Generate()
	newNamers.SecretsNettyNameBuilder.Prefix(customResource.Name).Base("netty").Suffix("secret").Generate()
	newNamers.LabelBuilder.Base(customResource.Name).Suffix("app").Generate()

	return &newNamers
}

func GetDefaultLabels(cr *brokerv1beta1.ActiveMQArtemis) map[string]string {
	defaultLabelData := selectors.LabelerData{}
	defaultLabelData.Base(cr.Name).Suffix("app").Generate()
	return defaultLabelData.Labels()
}

type StatefulSetInfo struct {
	NamespacedName types.NamespacedName
	Labels         map[string]string
}

//only test uses this
func NewReconcileActiveMQArtemis(c client.Client, s *runtime.Scheme) ActiveMQArtemisReconciler {
	return ActiveMQArtemisReconciler{
		Client: c,
		Scheme: s,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ActiveMQArtemisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&brokerv1beta1.ActiveMQArtemis{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Pod{})

	var err error
	controller, err := builder.Build(r)
	if err == nil {
		r.events = make(chan event.GenericEvent)
		err = controller.Watch(
			&source.Channel{Source: r.events},
			&handler.EnqueueRequestForObject{},
		)
	}
	return err
}
