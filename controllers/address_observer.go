package controllers

import (
	"context"
	"fmt"
	"time"

	ss "github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"

	jc "github.com/artemiscloud/activemq-artemis-operator/pkg/utils/jolokia_client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var olog = ctrl.Log.WithName("addressobserver_activemqartemisaddress")

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
	client client.Client,
	scheme *runtime.Scheme) *AddressObserver {

	observer := &AddressObserver{
		kubeclientset: kubeclientset,
		opclient:      client,
		opscheme:      scheme,
	}

	return observer
}

func (c *AddressObserver) Run(C chan types.NamespacedName, ctx context.Context) error {

	olog.Info("#### Started workers")

	for {
		select {
		case ready := <-C:
			olog.Info("address_observer received", "pod", ready)
			go c.newPodReady(&ready)
		case <-ctx.Done():
			olog.Info("address_observer received done on ctx, exiting event loop")
			return nil

		default:
			//log.Info("address_observer selected default, waiting a second")
			// NOTE: Sender will be blocked if this select is sleeping, might need decreased time here
			time.Sleep(500 * time.Millisecond)
		}
	}
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

	olog.Info("New pod ready.", "Pod", ready)

	//find out real name of the pod basename-(num - 1)
	//podBaseNames is our interested statefulsets name
	podBaseName, podSerial, labels := GetStatefulSetNameForPod(c.opclient, ready)
	if podBaseName == "" {
		olog.Info("Pod is not a candidate")
		return
	}

	realPodName := fmt.Sprintf("%s-%d", podBaseName, podSerial)

	podNamespacedName := types.NamespacedName{Namespace: ready.Namespace, Name: realPodName}
	pod := &corev1.Pod{}

	err1 := c.opclient.Get(context.TODO(), podNamespacedName, pod)
	if err1 != nil {
		olog.Error(err1, "Can't find the pod, abort", "pod", podNamespacedName)
		return
	}

	stsNameFromAnnotation := pod.Annotations[AnnotationStatefulSet]
	if stsNameFromAnnotation != "" {
		olog.Info("Ignoring drainer pod", "pod", realPodName)
		return
	}

	c.checkCRsForNewPod(pod, labels)
}

func (c *AddressObserver) checkCRsForNewPod(newPod *corev1.Pod, labels map[string]string) {
	//get the address cr instances
	addressInstances, err := c.getAddressInstances(newPod)
	if err != nil || len(addressInstances.Items) == 0 {
		olog.Info("No available address CRs")
		return
	}

	// go over each address instance for the new pod
	for _, a := range addressInstances.Items {
		//get the target namespaces
		targetCrNamespacedNames := createTargetCrNamespacedNames(newPod.Namespace, a.Spec.ApplyToCrNames)
		//e.g. ex-aao-ss
		podSSName, _ := c.getSSNameForPod(newPod)
		if podSSName == nil {
			olog.Info("Can't find pod's statefulset name", "pod", newPod.Name)
			continue
		}
		olog.Info("Got pod's ss name", "value", *podSSName)

		podCrName := namer.SSToCr(*podSSName)
		olog.Info("got pod's CR name", "value", podCrName)
		//if the new pod is a target for this address cr, create
		isTargetPod := false
		if targetCrNamespacedNames == nil {
			olog.Info("The new pod is the target as the address CR doesn't have applyToCrNames specified.")
			isTargetPod = true
		} else {

			newPodSSNamespacedName := types.NamespacedName{
				Name:      podCrName,
				Namespace: newPod.Namespace,
			}
			for _, ns := range targetCrNamespacedNames {
				if ns == newPodSSNamespacedName {
					olog.Info("The new pod is the target", "namespace", ns)
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

			ssInfos := ss.GetDeployedStatefulSetNames(c.opclient, []types.NamespacedName{podNamespacedName})
			jks := jc.GetBrokers(podNamespacedName, ssInfos, c.opclient)

			for _, jk := range jks {
				createAddressResource(jk, &a)
			}
		}
	}
}

func (c *AddressObserver) getSSNameForPod(newPod *corev1.Pod) (*string, *appsv1.StatefulSet) {

	if ownerRef := metav1.GetControllerOf(newPod); ownerRef != nil {
		sts := &appsv1.StatefulSet{}
		err := c.opclient.Get(context.TODO(), types.NamespacedName{Namespace: newPod.Namespace, Name: ownerRef.Name}, sts)
		if err != nil {
			olog.Error(err, "Error finding the statefulset")
			return nil, nil
		}
		return &ownerRef.Name, sts
	}
	return nil, nil
}

func (c *AddressObserver) getAddressInstances(newPod *corev1.Pod) (*brokerv1beta1.ActiveMQArtemisAddressList, error) {

	addrList := &brokerv1beta1.ActiveMQArtemisAddressList{}
	opts := []client.ListOption{
		client.InNamespace(newPod.Namespace),
	}
	if err := c.opclient.List(context.TODO(), addrList, opts...); err != nil {
		olog.Error(err, "failed to list address")
		return nil, err
	}

	return addrList, nil
}
