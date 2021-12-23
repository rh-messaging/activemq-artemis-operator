/*
Copyright 2017 The Kubernetes Authors.

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

package v2alpha3activemqartemisaddress

import (
	"context"
	"fmt"
	"time"

	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"

	mgmt "github.com/artemiscloud/activemq-artemis-management"
	v2alpha3 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha3"
	clientv2alpha3 "github.com/artemiscloud/activemq-artemis-operator/pkg/client/clientset/versioned/typed/broker/v2alpha3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//var log = logf.Log.WithName("addressobserver_v2alpha3activemqartemisaddress")

const AnnotationStatefulSet = "statefulsets.kubernetes.io/drainer-pod-owner"
const AnnotationDrainerPodTemplate = "statefulsets.kubernetes.io/drainer-pod-template"

type AddressObserver struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	opclient      client.Client
	opscheme      *runtime.Scheme
}

func NewAddressObserver(
	kubeclientset kubernetes.Interface,
	namespace string,
	client client.Client,
	scheme *runtime.Scheme) *AddressObserver {

	observer := &AddressObserver{
		kubeclientset: kubeclientset,
		opclient:      client,
		opscheme:      scheme,
	}

	return observer
}

func (c *AddressObserver) Run(C chan types.NamespacedName) error {

	log.Info("#### Started workers")

	for {
		select {
		case ready := <-C:
			// NOTE: c will be one ordinal higher than the real podname!
			log.Info("address_observer received C", "pod", ready)
			go c.newPodReady(&ready)
		default:
			//log.Info("address_observer selected default, waiting a second")
			// NOTE: Sender will be blocked if this select is sleeping, might need decreased time here
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

//if we support multiple statefulset in a namespace
//the pod may came from any stateful set
//so we need to check if the pod belongs to the statefulset(cr)
//that this address intends to be applied to
//that why we need one more property (targetBrokerCrName)
//and only apply addresses to those pods that in that SS.
//The property should be optional to keep backward compatibility
//and if multiple statefulsets exists while the property
//is not specified, apply to all pods)
func (c *AddressObserver) newPodReady(ready *types.NamespacedName) {

	log.Info("New pod ready.", "Pod", ready)

	//find out real name of the pod basename-(num - 1)
	//podBaseNames is our interested statefulsets name
	podBaseName, podSerial, labels := GetStatefulSetNameForPod(ready)
	if podBaseName == "" {
		log.Info("Pod is not a candidate")
		return
	}

	realPodName := fmt.Sprintf("%s-%d", podBaseName, podSerial-1)

	podNamespacedName := types.NamespacedName{ready.Namespace, realPodName}
	pod := &corev1.Pod{}

	err1 := c.opclient.Get(context.TODO(), podNamespacedName, pod)
	if err1 != nil {
		log.Error(err1, "Can't find the pod, abort", "pod", podNamespacedName)
		return
	}

	stsNameFromAnnotation := pod.Annotations[AnnotationStatefulSet]
	if stsNameFromAnnotation != "" {
		log.Info("Ignoring drainer pod", "pod", realPodName)
		return
	}

	c.checkCRsForNewPod(pod, labels)
}

func (c *AddressObserver) checkCRsForNewPod(newPod *corev1.Pod, labels map[string]string) {
	//get the address cr instances
	addressInstances, err := c.getAddressInstances(newPod)
	if err != nil || len(addressInstances.Items) == 0 {
		log.Info("No available address CRs")
		return
	}

	// go over each address instance for the new pod
	for _, a := range addressInstances.Items {
		//get the target namespaces
		targetCrNamespacedNames := createTargetCrNamespacedNames(newPod.Namespace, a.Spec.ApplyToCrNames)
		//e.g. ex-aao-ss
		podSSName, statefulset := c.getSSNameForPod(newPod)
		log.Info("Got pod's ss name", "value", *podSSName)
		if podSSName == nil {
			log.Info("Can't find pod's statefulset name", "pod", newPod.Name)
			continue
		}
		podCrName := namer.SSToCr(*podSSName)
		log.Info("got pod's CR name", "value", podCrName)
		//if the new pod is a target for this address cr, create
		isTargetPod := false
		if targetCrNamespacedNames == nil {
			log.Info("The new pod is the target as the address CR doesn't have applyToCrNames specified.")
			isTargetPod = true
		} else {

			newPodSSNamespacedName := types.NamespacedName{
				Name:      podCrName,
				Namespace: newPod.Namespace,
			}
			for _, ns := range targetCrNamespacedNames {
				if ns == newPodSSNamespacedName {
					log.Info("The new pod is the target", "namespace", ns)
					isTargetPod = true
					break
				}
			}
		}
		if isTargetPod {
			podNamespacedName := types.NamespacedName{
				Name:      newPod.Name,
				Namespace: newPod.Namespace,
			}
			var jolokiaSecretName string = podCrName + "-jolokia-secret"
			log.Info("Recreating address resources on new Pod", "Name", newPod.Name, "secret name", jolokiaSecretName)
			jolokiaUser, jolokiaPassword, jolokiaProtocol := resolveJolokiaRequestParams(newPod.Namespace,
				&a, c.opclient, c.opscheme, jolokiaSecretName, &newPod.Spec.Containers, podNamespacedName, statefulset, labels)

			log.Info("New Jolokia with ", "User: ", jolokiaUser, "Protocol: ", jolokiaProtocol)
			artemis := mgmt.GetArtemis(newPod.Status.PodIP, "8161", "amq-broker", jolokiaUser, jolokiaPassword, jolokiaProtocol)
			createAddressResource(artemis, &a)
		}
	}
}

func (c *AddressObserver) getSSNameForPod(newPod *corev1.Pod) (*string, *appsv1.StatefulSet) {

	if ownerRef := metav1.GetControllerOf(newPod); ownerRef != nil {
		sts := &appsv1.StatefulSet{}
		err := c.opclient.Get(context.TODO(), types.NamespacedName{newPod.Namespace, ownerRef.Name}, sts)
		if err != nil {
			log.Error(err, "Error finding the statefulset")
			return nil, nil
		}
		return &ownerRef.Name, sts
	}
	return nil, nil
}

func (c *AddressObserver) getAddressInstances(newPod *corev1.Pod) (*v2alpha3.ActiveMQArtemisAddressList, error) {

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		//log error here
		return nil, err
	}

	brokerClient, err2 := clientv2alpha3.NewForConfig(cfg)
	if err2 != nil {
		log.Error(err2, "Error building brokerclient: %s", err2.Error())
		return nil, err2
	}

	addressInterface := brokerClient.ActiveMQArtemisAddresses(newPod.Namespace)
	result, listerr := addressInterface.List(metav1.ListOptions{})
	if listerr != nil {
		log.Error(listerr, "Failed to get address resources")
		return nil, listerr
	}

	return result, nil
}
