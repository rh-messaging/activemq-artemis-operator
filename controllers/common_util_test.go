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
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

var chars = []rune("hgjkmnpqrtvwxyzslbcdaefiou")

func randStringWithPrefix(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	length := 6
	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func randString() string {
	return randStringWithPrefix("br-")
}

func CleanResourceWithTimeouts(res client.Object, name string, namespace string, cleanTimeout time.Duration, cleanInterval time.Duration) {
	Expect(k8sClient.Delete(ctx, res)).Should(Succeed())
	By("make sure resource is gone")
	Eventually(func(g Gomega) {
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, res)
		g.Expect(errors.IsNotFound(err)).To(BeTrue())
	}, cleanTimeout, cleanInterval).Should(Succeed())
}

func CleanResource(res client.Object, name string, namespace string) {
	CleanResourceWithTimeouts(res, name, namespace, timeout, interval)
}

func CleanClusterResource(res client.Object, name string, namespace string) {
	CleanResourceWithTimeouts(res, name, namespace, existingClusterTimeout, existingClusterInterval)
}

func checkSecretHasCorrectKeyValue(g Gomega, secName string, ns types.NamespacedName, key string, expectedValue string) {
	g.Eventually(func(g Gomega) {
		secret, err := secrets.RetriveSecret(ns, secName, make(map[string]string), k8sClient)
		g.Expect(err).Should(BeNil())
		data := secret.Data[key]
		g.Expect(strings.Contains(string(data), expectedValue)).Should(BeTrue())
	}, timeout, interval).Should(Succeed())
}

func hexShaHashOfMap(props []string) string {
	return hex.EncodeToString(alder32Of(props))
}

func CurrentSpecShortName() string {

	name := path.Base(CurrentSpecReport().LeafNodeLocation.FileName)
	name = strings.ReplaceAll(name, "activemqartemis", "aa")
	name = strings.ReplaceAll(name, "deploy_operator", "do")
	name = strings.ReplaceAll(name, "_test", "")
	name = strings.ReplaceAll(name, ".go", "")
	name = strings.ReplaceAll(name, "_", "-")

	lineNumber := strconv.Itoa(CurrentSpecReport().LeafNodeLocation.LineNumber)

	nameLimit := specShortNameLimit - len(lineNumber)

	if len(name) > nameLimit {
		nameTokens := strings.Split(name, "-")
		name = nameTokens[0]
		for i := 1; i < len(nameTokens) && len(name) < nameLimit; i++ {
			if len(nameTokens[i]) > 3 {
				name += "-" + nameTokens[i][0:3]
			} else if len(nameTokens[i]) > 0 {
				name += "-" + nameTokens[i]
			}
		}
	}

	if len(name) > nameLimit {
		name = name[0:nameLimit]
	}

	name += lineNumber

	return name
}

// The spec resource names are based on current spec short name which has
// max 25 characters (see specShortNameLimit) because the maximum service
// name length is 63 characters.
func NextSpecResourceName() string {
	// The resCount is converted to a letter(97+resCount%25) and appened
	// to the current spec short name to generate a unique resource name.
	// The rune type is an alias for int32 and it is used to distinguish
	// character values from integer values.
	name := CurrentSpecShortName() + string(rune(97+resCount%25))
	resCount++

	return name
}

func newArtemisSpecWithFastProbes() brokerv1beta1.ActiveMQArtemisSpec {
	spec := brokerv1beta1.ActiveMQArtemisSpec{}

	// sensible fast defaults for tests against existing cluster
	spec.DeploymentPlan.LivenessProbe = &corev1.Probe{
		InitialDelaySeconds: 1,
		PeriodSeconds:       2,
	}
	spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
		InitialDelaySeconds: 1,
		PeriodSeconds:       2,
	}

	return spec
}

func generateArtemisSpec(namespace string) brokerv1beta1.ActiveMQArtemis {

	toCreate := brokerv1beta1.ActiveMQArtemis{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemis",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      NextSpecResourceName(),
			Namespace: namespace,
		},
		Spec: newArtemisSpecWithFastProbes(),
	}

	return toCreate
}

func DeployCustomBroker(targetNamespace string, customFunc func(candidate *brokerv1beta1.ActiveMQArtemis)) (*brokerv1beta1.ActiveMQArtemis, *brokerv1beta1.ActiveMQArtemis) {
	ctx := context.Background()
	brokerCrd := generateArtemisSpec(targetNamespace)

	brokerCrd.Spec.DeploymentPlan.Size = common.Int32ToPtr(1)

	if customFunc != nil {
		customFunc(&brokerCrd)
	}

	Expect(k8sClient.Create(ctx, &brokerCrd)).Should(Succeed())

	createdBrokerCrd := brokerv1beta1.ActiveMQArtemis{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(brokerCrd.Name, targetNamespace, &createdBrokerCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdBrokerCrd.Name).Should(Equal(brokerCrd.ObjectMeta.Name))
	Expect(createdBrokerCrd.Namespace).Should(Equal(targetNamespace))

	return &brokerCrd, &createdBrokerCrd
}

func getPersistedVersionedCrd(name string, nameSpace string, object client.Object) bool {
	key := types.NamespacedName{Name: name, Namespace: nameSpace}
	err := k8sClient.Get(ctx, key, object)
	return err == nil
}

func DeploySecret(targetNamespace string, customFunc func(candidate *corev1.Secret)) (*corev1.Secret, *corev1.Secret) {
	ctx := context.Background()

	secretName := NextSpecResourceName()
	secretDefinition := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: targetNamespace,
		},
	}

	if customFunc != nil {
		customFunc(&secretDefinition)
	}

	Expect(k8sClient.Create(ctx, &secretDefinition)).Should(Succeed())

	createdSecret := corev1.Secret{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(secretDefinition.Name, targetNamespace, &createdSecret)
	}, timeout, interval).Should(BeTrue())
	Expect(createdSecret.Name).Should(Equal(secretDefinition.ObjectMeta.Name))
	Expect(createdSecret.Namespace).Should(Equal(targetNamespace))

	return &secretDefinition, &createdSecret
}

func generateOriginalArtemisSpec(namespace string, name string) *brokerv1beta1.ActiveMQArtemis {

	spec := newArtemisSpecWithFastProbes()

	toCreate := brokerv1beta1.ActiveMQArtemis{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemis",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}

	return &toCreate
}

func DeployBroker(brokerName string, targetNamespace string) (*brokerv1beta1.ActiveMQArtemis, *brokerv1beta1.ActiveMQArtemis) {
	ctx := context.Background()
	brokerCrd := generateOriginalArtemisSpec(targetNamespace, brokerName)

	Expect(k8sClient.Create(ctx, brokerCrd)).Should(Succeed())

	createdBrokerCrd := &brokerv1beta1.ActiveMQArtemis{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(brokerCrd.ObjectMeta.Name, targetNamespace, createdBrokerCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdBrokerCrd.Name).Should(Equal(createdBrokerCrd.ObjectMeta.Name))

	return brokerCrd, createdBrokerCrd

}

func DeploySecurity(secName string, targetNamespace string, customFunc func(candidate *brokerv1beta1.ActiveMQArtemisSecurity)) (*brokerv1beta1.ActiveMQArtemisSecurity, *brokerv1beta1.ActiveMQArtemisSecurity) {
	ctx := context.Background()
	secCrd := generateSecuritySpec(secName, targetNamespace)

	brokerDomainName := "activemq"
	loginModuleName := "module1"
	loginModuleFlag := "sufficient"
	okDefaultPwd := "ok"

	loginModuleList := make([]brokerv1beta1.PropertiesLoginModuleType, 1)
	propLoginModule := brokerv1beta1.PropertiesLoginModuleType{
		Name: loginModuleName,
		Users: []brokerv1beta1.UserType{
			{
				Name:     "user1",
				Password: &okDefaultPwd,
				Roles: []string{
					"admin", "amq",
				},
			},
		},
	}
	loginModuleList = append(loginModuleList, propLoginModule)
	secCrd.Spec.LoginModules.PropertiesLoginModules = loginModuleList

	secCrd.Spec.SecurityDomains.BrokerDomain = brokerv1beta1.BrokerDomainType{
		Name: &brokerDomainName,
		LoginModules: []brokerv1beta1.LoginModuleReferenceType{
			{
				Name: &loginModuleName,
				Flag: &loginModuleFlag,
			},
		},
	}

	customFunc(secCrd)

	Expect(k8sClient.Create(ctx, secCrd)).Should(Succeed())

	createdSecCrd := &brokerv1beta1.ActiveMQArtemisSecurity{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(secCrd.ObjectMeta.Name, targetNamespace, createdSecCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdSecCrd.Name).Should(Equal(secCrd.ObjectMeta.Name))

	return secCrd, createdSecCrd
}

func generateSecuritySpec(secName string, targetNamespace string) *brokerv1beta1.ActiveMQArtemisSecurity {

	spec := brokerv1beta1.ActiveMQArtemisSecuritySpec{}

	theName := secName
	if secName == "" {
		theName = randString()
	}

	toCreate := brokerv1beta1.ActiveMQArtemisSecurity{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemisSecurity",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      theName,
			Namespace: targetNamespace,
		},
		Spec: spec,
	}

	return &toCreate
}

func RunCommandInPod(podName string, containerName string, command []string) (*string, error) {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}
	restClient, err := apiutil.RESTClientForGVK(gvk, false, testEnv.Config, serializer.NewCodecFactory(testEnv.Scheme))
	Expect(err).To(BeNil())
	execReq := restClient.
		Post().
		Namespace(defaultNamespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
		}, runtime.NewParameterCodec(testEnv.Scheme))

	exec, err := remotecommand.NewSPDYExecutor(testEnv.Config, "POST", execReq.URL())

	if err != nil {
		return nil, err
	}

	var consumerCapturedOut bytes.Buffer

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: &consumerCapturedOut,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return nil, err
	}

	//try get some content if any
	Eventually(func(g Gomega) {
		g.Expect(consumerCapturedOut.Len() > 0)
	}, existingClusterTimeout, interval)

	content := consumerCapturedOut.String()

	return &content, nil
}

func LogsOfPod(podWithOrdinal string, brokerName string, namespace string, g Gomega) string {

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}
	restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme.Scheme))
	g.Expect(err).To(BeNil())

	readCloser, err := restClient.
		Get().
		Namespace(namespace).
		Resource("pods").
		Name(podWithOrdinal).
		SubResource("log").
		VersionedParams(&corev1.PodLogOptions{
			Container: brokerName + "-container",
		}, runtime.NewParameterCodec(scheme.Scheme)).Stream(context.TODO())
	g.Expect(err).To(BeNil())

	defer readCloser.Close()

	result, err := io.ReadAll(readCloser)
	g.Expect(err).To(BeNil())

	return string(result)
}

func ExecOnPod(podWithOrdinal string, brokerName string, namespace string, command []string, g Gomega) string {

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}
	restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme.Scheme))
	g.Expect(err).To(BeNil())

	execReq := restClient.
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(podWithOrdinal).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: brokerName + "-container",
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
		}, runtime.NewParameterCodec(scheme.Scheme))

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", execReq.URL())
	g.Expect(err).To(BeNil())

	var outPutbuffer bytes.Buffer

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  os.Stdin,
		Stdout: &outPutbuffer,
		Stderr: os.Stderr,
		Tty:    false,
	})
	g.Expect(err).To(BeNil())

	g.Eventually(func(g Gomega) {
		By("Checking for output from " + fmt.Sprintf(" command: %v", command))
		g.Expect(outPutbuffer.Len() > 0)
		if verbose {
			fmt.Printf("\n%v %v resulted in %s\n", time.Now(), command, outPutbuffer.String())
		}
	}, timeout, interval*5).Should(Succeed())

	return outPutbuffer.String()
}

func GenerateAddressSpec(name string, ns string, address string, queue string, isMulticast bool, autoDelete bool) *brokerv1beta1.ActiveMQArtemisAddress {

	spec := brokerv1beta1.ActiveMQArtemisAddressSpec{}

	spec.AddressName = address
	spec.QueueName = &queue

	routingType := "anycast"
	if isMulticast {
		routingType = "multicast"
	}
	spec.RoutingType = &routingType

	toCreate := &brokerv1beta1.ActiveMQArtemisAddress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ActiveMQArtemisAddress",
			APIVersion: brokerv1beta1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: spec,
	}

	return toCreate
}

func DeployAddress(candidate *brokerv1beta1.ActiveMQArtemisAddress) {
	ctx := context.Background()

	Expect(k8sClient.Create(ctx, candidate)).Should(Succeed())

	createdAddressCrd := &brokerv1beta1.ActiveMQArtemisAddress{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(candidate.ObjectMeta.Name, candidate.Namespace, createdAddressCrd)
	}, timeout, interval).Should(BeTrue())
	Expect(createdAddressCrd.Name).Should(Equal(candidate.ObjectMeta.Name))
}

func DeployCustomAddress(targetNamespace string, customFunc func(candidate *brokerv1beta1.ActiveMQArtemisAddress)) (*brokerv1beta1.ActiveMQArtemisAddress, *brokerv1beta1.ActiveMQArtemisAddress) {

	ctx := context.Background()
	addressCr := GenerateAddressSpec(NextSpecResourceName(), targetNamespace, "myAddress", "myQueue", false, true)

	if customFunc != nil {
		customFunc(addressCr)
	}

	Expect(k8sClient.Create(ctx, addressCr)).Should(Succeed())

	createdAddressCr := brokerv1beta1.ActiveMQArtemisAddress{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(addressCr.Name, targetNamespace, &createdAddressCr)
	}, timeout, interval).Should(BeTrue())
	Expect(createdAddressCr.Name).Should(Equal(addressCr.Name))
	Expect(createdAddressCr.Namespace).Should(Equal(targetNamespace))

	return addressCr, &createdAddressCr
}

func GetOperatorLog(ns string) (*string, error) {
	cfg, err := config.GetConfig()
	Expect(err).To(BeNil())
	labelSelector, err := labels.Parse("control-plane=controller-manager")
	Expect(err).To(BeNil())
	clientset, err := kubernetes.NewForConfig(cfg)
	Expect(err).To(BeNil())
	listOpts := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	podList, err := clientset.CoreV1().Pods(ns).List(ctx, listOpts)
	Expect(err).To(BeNil())
	Expect(len(podList.Items)).To(Equal(1))
	operatorPod := podList.Items[0]

	podLogOpts := corev1.PodLogOptions{}
	req := clientset.CoreV1().Pods(ns).GetLogs(operatorPod.Name, &podLogOpts)
	podLogs, err := req.Stream(context.Background())
	Expect(err).To(BeNil())
	defer podLogs.Close()

	Expect(err).To(BeNil())

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	Expect(err).To(BeNil())
	str := buf.String()

	return &str, nil
}
