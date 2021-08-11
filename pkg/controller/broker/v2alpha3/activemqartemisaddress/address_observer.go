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

	mgmt "github.com/artemiscloud/activemq-artemis-management"
	clientv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/client/clientset/versioned/typed/broker/v2alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//var log = logf.Log.WithName("addressobserver_v2alpha1activemqartemisaddress")

const AnnotationStatefulSet = "statefulsets.kubernetes.io/drainer-pod-owner"
const AnnotationDrainerPodTemplate = "statefulsets.kubernetes.io/drainer-pod-template"

type AddressObserver struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	opclient      client.Client
}

func NewAddressObserver(
	kubeclientset kubernetes.Interface,
	namespace string,
	client client.Client) *AddressObserver {

	observer := &AddressObserver{
		kubeclientset: kubeclientset,
		opclient:      client,
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
//and return error if multiple statefulsets exists while the property
//is not specified. (or apply to all pods)
func (c *AddressObserver) newPodReady(ready *types.NamespacedName) {

	log.Info("New pod ready.", "Pod", ready)

	//find out real name of the pod basename-(num - 1)
	//podBaseNames is our interested statefulsets name
	podBaseName, podSerial := GetStatefulSetNameForPod(ready)
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

	c.checkCRsForNewPod(pod)
}

func (c *AddressObserver) checkCRsForNewPod(newPod *corev1.Pod) {

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		log.Error(err, "Error building kubeconfig: %s", err.Error())
	}

	brokerClient, err2 := clientv2alpha1.NewForConfig(cfg)
	if err2 != nil {
		log.Error(err2, "Error building brokerclient: %s", err2.Error())
	}

	addressInterface := brokerClient.ActiveMQArtemisAddresses((*newPod).Namespace)
	result, listerr := addressInterface.List(metav1.ListOptions{})
	if listerr != nil {
		log.Error(listerr, "Failed to get address resources")
		return
	}

	if len(result.Items) == 0 {
		log.Info("No pre-installed CRs so nothing to do")
		return
	}

	if ownerRef := metav1.GetControllerOf(newPod); ownerRef != nil {

		podNamespacedName := types.NamespacedName{newPod.Name, newPod.Namespace}

		sts := &appsv1.StatefulSet{}

		err := c.opclient.Get(context.TODO(), types.NamespacedName{newPod.Namespace, ownerRef.Name}, sts)
		if err != nil {
			log.Error(err, "Error finding the statefulset")
		}

		containers := newPod.Spec.Containers
		var jolokiaUser string
		var jolokiaPassword string
		if len(containers) == 1 {
			envVars := containers[0].Env
			for _, oneVar := range envVars {
				if "AMQ_USER" == oneVar.Name {
					jolokiaUser = getEnvVarValue(&oneVar, &podNamespacedName, sts, c.opclient)
				}
				if "AMQ_PASSWORD" == oneVar.Name {
					jolokiaPassword = getEnvVarValue(&oneVar, &podNamespacedName, sts, c.opclient)
				}
				if jolokiaUser != "" && jolokiaPassword != "" {
					break
				}
			}
		}

		log.Info("New Jolokia with ", "User: ", jolokiaUser, "podIP: ", newPod.Status.PodIP)
		artemis := mgmt.NewArtemis(newPod.Status.PodIP, "8161", "amq-broker", jolokiaUser, jolokiaPassword)

		for _, a := range result.Items {
			log.Info("Creating pre-installed address", "address", a.Spec.AddressName, "queue", a.Spec.QueueName, "routing type", a.Spec.RoutingType)

			_, err := artemis.CreateQueue(a.Spec.AddressName, a.Spec.QueueName, a.Spec.RoutingType)
			if nil != err {
				log.Error(err, "Error creating address")
			} else {
				log.Info("Successfully Created ActiveMQArtemisAddress")
			}
		}
	}
}
