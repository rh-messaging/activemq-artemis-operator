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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	nsoptions "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/namespaces"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/lsrcrs"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/selectors"
)

var clog = ctrl.Log.WithName("controller_v1beta1activemqartemis")

var namespacedNameToFSM = make(map[types.NamespacedName]*ActiveMQArtemisFSM)

type ActiveMQArtemisConfigHandler interface {
	IsApplicableFor(brokerNamespacedName types.NamespacedName) bool
	Config(initContainers []corev1.Container, outputDirRoot string, yacfgProfileVersion string, yacfgProfileName string) (value []string)
}

var namespaceToConfigHandler = make(map[types.NamespacedName]ActiveMQArtemisConfigHandler)

func GetBrokerConfigHandler(brokerNamespacedName types.NamespacedName) (handler ActiveMQArtemisConfigHandler) {
	for _, handler := range namespaceToConfigHandler {
		if handler.IsApplicableFor(brokerNamespacedName) {
			return handler
		}
	}
	return nil
}

func UpdatePodForSecurity(securityHandlerNamespacedName types.NamespacedName, handler ActiveMQArtemisConfigHandler) error {
	success := true
	for nsn, fsm := range namespacedNameToFSM {
		if handler.IsApplicableFor(nsn) {
			fsm.SetPodInvalid(true)
			glog.Info("Need update fsm for security", "fsm", nsn)
			if err, _ := fsm.Update(); err != nil {
				success = false
				glog.Error(err, "error in updating security", "cr", fsm.namespacedName)
			}
		}
	}
	if success {
		return nil
	}
	err := fmt.Errorf("error in update security, please see log for details")
	return err
}

func RemoveBrokerConfigHandler(namespacedName types.NamespacedName) {
	glog.Info("Removing config handler", "name", namespacedName)
	oldHandler, ok := namespaceToConfigHandler[namespacedName]
	if ok {
		delete(namespaceToConfigHandler, namespacedName)
		glog.Info("Handler removed, updating fsm if exists")
		UpdatePodForSecurity(namespacedName, oldHandler)
	}
}

func AddBrokerConfigHandler(namespacedName types.NamespacedName, handler ActiveMQArtemisConfigHandler, toReconcile bool) error {
	if _, ok := namespaceToConfigHandler[namespacedName]; ok {
		glog.V(1).Info("There is an old config handler, it'll be replaced")
	}
	namespaceToConfigHandler[namespacedName] = handler
	glog.V(1).Info("A new config handler has been added", "handler", handler)
	if toReconcile {
		glog.V(1).Info("Updating broker security")
		return UpdatePodForSecurity(namespacedName, handler)
	}
	return nil
}

// ActiveMQArtemisReconciler reconciles a ActiveMQArtemis object
type ActiveMQArtemisReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Result ctrl.Result
}

//run 'make manifests' after changing the following rbac markers

//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemises,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemises/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=broker.amq.io,resources=activemqartemises/finalizers,verbs=update
//+kubebuilder:rbac:groups=broker.amq.io,resources=pods,verbs=get;list
//+kubebuilder:rbac:groups="",resources=pods;services;endpoints;persistentvolumeclaims;events;configmaps;secrets;routes,verbs=*
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=*
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes;routes/custom-host;routes/status,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;create
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

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
	reqLogger := ctrl.Log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ActiveMQArtemis")

	if !nsoptions.Match(request.Namespace) {
		reqLogger.Info("Request not in watch list, ignore", "request", request)
		return reconcile.Result{}, nil
	}

	var err error = nil
	var namespacedNameFSM *ActiveMQArtemisFSM = nil
	var amqbfsm *ActiveMQArtemisFSM = nil

	customResource := &brokerv1beta1.ActiveMQArtemis{}
	namespacedName := types.NamespacedName{
		Name:      request.Name,
		Namespace: request.Namespace,
	}

	// Fetch the ActiveMQArtemis instance
	// When first creating this will have err == nil
	// When deleting after creation this will have err NotFound
	// When deleting before creation reconcile won't be called
	if err = r.Get(context.TODO(), request.NamespacedName, customResource); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("ActiveMQArtemis Controller Reconcile encountered a IsNotFound, checking to see if we should delete namespacedName tracking for request NamespacedName " + request.NamespacedName.String())

			// See if we have been tracking this NamespacedName
			if namespacedNameFSM = namespacedNameToFSM[namespacedName]; namespacedNameFSM != nil {
				reqLogger.Info("Removing namespacedName tracking for " + namespacedName.String())
				// If so we should no longer track it
				amqbfsm = namespacedNameToFSM[namespacedName]
				//remove the fsm secret
				lsrcrs.DeleteLastSuccessfulReconciledCR(request.NamespacedName, "broker", amqbfsm.namers.LabelBuilder.Labels(), r.Client)
				amqbfsm.Exit()
				delete(namespacedNameToFSM, namespacedName)
				amqbfsm = nil
			}

			// Setting err to nil to prevent requeue
			err = nil
		} else {
			reqLogger.Error(err, "ActiveMQArtemis Controller Reconcile errored thats not IsNotFound, requeuing request", "Request Namespace", request.Namespace, "Request Name", request.Name)
			// Leaving err as !nil causes requeue
		}

		// Add error detail for use later
		return r.Result, err
	}

	// Do lookup to see if we have a fsm for the incoming name in the incoming namespace
	// if not, create it
	// for the given fsm, do an update
	// - update first level sets? what if the operator has gone away and come back? stateless?
	if namespacedNameFSM = namespacedNameToFSM[namespacedName]; namespacedNameFSM == nil {
		reqLogger.Info("Didn't find fsm for the CR, try to search history", "requested", namespacedName)
		//try to retrieve last successful reconciled CR
		lsrcr := lsrcrs.RetrieveLastSuccessfulReconciledCR(namespacedName, "broker", r.Client, GetDefaultLabels(customResource))
		if lsrcr != nil {
			reqLogger.Info("There is a LastSuccessfulReconciledCR")
			//restoring fsm
			var fsmData ActiveMQArtemisFSMData
			var fsm *ActiveMQArtemisFSM
			if merr := common.FromJson(&lsrcr.Data, &fsmData); merr != nil {
				reqLogger.Error(merr, "failed to unmarshal fsm, create a new one")
				fsm = MakeActiveMQArtemisFSM(customResource, namespacedName, r)
			} else {
				reqLogger.Info("recreate fsm from data")
				storedCR := brokerv1beta1.ActiveMQArtemis{}
				merr := common.FromJson(&lsrcr.CR, &storedCR)
				if merr != nil {
					reqLogger.Error(merr, "failed to unmarshal cr, using existing one")
					fsm = MakeActiveMQArtemisFSMFromData(&fsmData, customResource, namespacedName, r)
				} else {
					reqLogger.Info("Restoring fsm")
					fsm = MakeActiveMQArtemisFSMFromData(&fsmData, &storedCR, namespacedName, r)
				}
			}
			namespacedNameToFSM[namespacedName] = fsm
			if lsrcr.Checksum == customResource.ResourceVersion {
				//this is an operator restart. Don't do reconcile
				reqLogger.Info("Detected possible operator restart with no broker CR changes", "res", customResource.ResourceVersion)
				return r.Result, nil
			}
			reqLogger.Info("A new version of CR comes in", "old", lsrcr.Checksum, "new", customResource.ResourceVersion)
		}
	}

	if namespacedNameFSM = namespacedNameToFSM[namespacedName]; namespacedNameFSM == nil {

		amqbfsm = MakeActiveMQArtemisFSM(customResource, namespacedName, r)
		namespacedNameToFSM[namespacedName] = amqbfsm

		// Enter the first state; atm CreatingK8sResourcesState
		amqbfsm.Enter(CreatingK8sResourcesID)
	} else {
		amqbfsm = namespacedNameFSM
		//remember current customeResource so that we can compare for update
		amqbfsm.UpdateCustomResource(customResource)

		err, _ = amqbfsm.Update()
	}

	//persist the CR
	if err == nil {
		fsmData := amqbfsm.GetFSMData()
		fsmstr, merr := common.ToJson(&fsmData)
		if merr != nil {
			reqLogger.Error(merr, "failed to marshal fsm")
		}
		crstr, merr := common.ToJson(customResource)
		if merr != nil {
			reqLogger.Error(merr, "failed to marshal cr")
		}
		lsrcrs.StoreLastSuccessfulReconciledCR(customResource, customResource.Name,
			customResource.Namespace, "broker", crstr, fsmstr, customResource.ResourceVersion,
			amqbfsm.namers.LabelBuilder.Labels(), r.Client, r.Scheme)
	}
	// Single exit, return the result and error condition
	return r.Result, err
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

//get the statefulset names
func GetDeployedStatefuleSetNames(targetCrNames []types.NamespacedName) []StatefulSetInfo {

	var result []StatefulSetInfo = nil

	if len(targetCrNames) == 0 {
		for _, fsm := range namespacedNameToFSM {
			info := StatefulSetInfo{
				NamespacedName: fsm.GetStatefulSetNamespacedName(),
				Labels:         fsm.namers.LabelBuilder.Labels(),
			}
			result = append(result, info)
		}
		return result
	}

	for _, target := range targetCrNames {
		clog.Info("Trying to get target fsm", "target", target)
		if fsm := namespacedNameToFSM[target]; fsm != nil {
			clog.Info("got fsm", "fsm", fsm, "ss namer", fsm.namers.SsNameBuilder.Name())
			info := StatefulSetInfo{
				NamespacedName: fsm.GetStatefulSetNamespacedName(),
				Labels:         fsm.namers.LabelBuilder.Labels(),
			}
			result = append(result, info)
		}
	}
	return result
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&brokerv1beta1.ActiveMQArtemis{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
