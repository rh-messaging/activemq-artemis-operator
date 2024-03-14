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
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	brokerv1beta1 "github.com/artemiscloud/activemq-artemis-operator/api/v1beta1"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/secrets"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/common"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/utils/namer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"software.sslmate.com/src/go-pkcs12"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

var chars = []rune("hgjkmnpqrtvwxyzslbcdaefiou")
var defaultPassword string = "password"
var defaultSanDnsNames = []string{"*.apps.artemiscloud.io", "*.tests.artemiscloud.io"}
var okDefaultPwd = "okdefaultpassword"

type TestLogWriter struct {
	unbufferedWriter bytes.Buffer
}

func (w *TestLogWriter) Write(p []byte) (n int, err error) {
	num, err := w.unbufferedWriter.Write(p)
	if err != nil {
		return num, err
	}
	return GinkgoWriter.Write(p)
}

func (w *TestLogWriter) StartLogging() {
	w.unbufferedWriter = *bytes.NewBuffer(nil)
}

func (w *TestLogWriter) StopLogging() {
	w.unbufferedWriter.Reset()
}

var TestLogWrapper = TestLogWriter{}

func MatchPattern(content string, pattern string) (matched bool, err error) {
	return regexp.Match(pattern, []byte(content))
}

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
	spec.DeploymentPlan.ReadinessProbe = &corev1.Probe{
		InitialDelaySeconds: 1,
		PeriodSeconds:       3,
	}
	spec.DeploymentPlan.LivenessProbe = &corev1.Probe{
		InitialDelaySeconds: 6,
		PeriodSeconds:       3,
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

	if customFunc != nil {
		customFunc(secCrd)
	}

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
	return RunCommandInPodWithNamespace(podName, defaultNamespace, containerName, command)
}

func RunCommandInPodWithNamespace(podName string, podNamespace string, containerName string, command []string) (*string, error) {
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}
	httpClient, err := rest.HTTPClientFor(restConfig)
	Expect(err).To(BeNil())
	restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme.Scheme), httpClient)
	Expect(err).To(BeNil())
	execReq := restClient.
		Post().
		Namespace(podNamespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
		}, runtime.NewParameterCodec(scheme.Scheme))

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", execReq.URL())

	if err != nil {
		return nil, err
	}

	var consumerCapturedOut bytes.Buffer

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
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

func EventsOfPod(podWithOrdinal string, namespace string, g Gomega) *corev1.EventList {

	cfg, err := config.GetConfig()
	g.Expect(err).To(BeNil())

	clientset, err := kubernetes.NewForConfig(cfg)
	g.Expect(err).To(BeNil())

	events, err := clientset.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + podWithOrdinal,
		TypeMeta:      metav1.TypeMeta{Kind: "Pod"},
	})
	g.Expect(err).To(BeNil())

	return events
}

func LogsOfPod(podWithOrdinal string, brokerName string, namespace string, g Gomega) string {

	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}
	httpClient, err := rest.HTTPClientFor(restConfig)
	Expect(err).To(BeNil())
	restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme.Scheme), httpClient)
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
	httpClient, err := rest.HTTPClientFor(restConfig)
	g.Expect(err).To(BeNil())
	restClient, err := apiutil.RESTClientForGVK(gvk, false, restConfig, serializer.NewCodecFactory(scheme.Scheme), httpClient)
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

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
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

func NewPriveKey() (*rsa.PrivateKey, error) {
	caPrivKey, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	if err := caPrivKey.Validate(); err != nil {
		return nil, err
	}
	return caPrivKey, nil
}

// generate a keystore file in bytes
// the keystore contains a self signed cert
func GenerateKeystore(password string, dnsNames []string) ([]byte, error) {
	// create the key pair
	caPrivKey, err := NewPriveKey()
	if err != nil {
		return nil, err
	}
	if err := caPrivKey.Validate(); err != nil {
		return nil, err
	}

	// set up our CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(202305071030),
		Subject: pkix.Name{
			CommonName:         "ArtemisCloud Broker",
			OrganizationalUnit: []string{"Broker"},
			Organization:       []string{"ArtemisCloud"},
		},
		NotBefore:          time.Now(),
		NotAfter:           time.Now().AddDate(10, 0, 0),
		IsCA:               false,
		SignatureAlgorithm: x509.SHA256WithRSA,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if len(dnsNames) > 0 {
		// Subject Alternative Names
		ca.DNSNames = dnsNames
	}

	// create the self-signed CA
	caBytes, err := x509.CreateCertificate(crand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	cert, err := x509.ParseCertificate(caBytes)
	if err != nil {
		return nil, err
	}

	ksBytes, err := pkcs12.Encode(crand.Reader, caPrivKey, cert, []*x509.Certificate{}, password)
	if err != nil {
		return nil, err
	}

	return ksBytes, nil
}

func GenerateTrustStoreFromKeyStore(ksBytes []byte, password string) ([]byte, error) {

	_, cert, _, err := pkcs12.DecodeChain(ksBytes, password)

	if err != nil {
		return nil, err
	}

	pfxBytes, err := pkcs12.EncodeTrustStore(crand.Reader, []*x509.Certificate{cert}, password)

	if err != nil {
		return nil, err
	}

	return pfxBytes, nil
}

func CreateTlsSecret(secretName string, ns string, ksPassword string, nsNames []string) (*corev1.Secret, error) {

	certData := make(map[string][]byte)
	stringData := make(map[string]string)

	brokerKs, ferr := GenerateKeystore(ksPassword, nsNames)
	if ferr != nil {
		return nil, ferr
	}
	clientTs, ferr := GenerateTrustStoreFromKeyStore(brokerKs, ksPassword)
	if ferr != nil {
		return nil, ferr
	}

	certData["broker.ks"] = brokerKs
	certData["client.ts"] = clientTs
	stringData["keyStorePassword"] = ksPassword
	stringData["trustStorePassword"] = ksPassword

	tlsSecret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ns,
		},
		Data:       certData,
		StringData: stringData,
	}
	return &tlsSecret, nil
}

func StringToPtr(v string) *string {
	return &v
}

func DeployCustomPVC(name string, targetNamespace string, customFunc func(candidate *corev1.PersistentVolumeClaim)) (*corev1.PersistentVolumeClaim, *corev1.PersistentVolumeClaim) {
	ctx := context.Background()
	pvc := corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: targetNamespace,
		},
	}

	if customFunc != nil {
		customFunc(&pvc)
	}

	Expect(k8sClient.Create(ctx, &pvc)).Should(Succeed())

	createdPvc := corev1.PersistentVolumeClaim{}

	Eventually(func() bool {
		return getPersistedVersionedCrd(pvc.Name, targetNamespace, &createdPvc)
	}, timeout, interval).Should(BeTrue())
	Expect(createdPvc.Name).Should(Equal(pvc.Name))
	Expect(createdPvc.Namespace).Should(Equal(targetNamespace))

	return &pvc, &createdPvc
}

func WaitForPod(crName string, iPods ...int32) {
	ssKey := types.NamespacedName{
		Name:      namer.CrToSS(crName),
		Namespace: defaultNamespace,
	}

	currentSS := &appsv1.StatefulSet{}

	for podOrdinal := range iPods {
		podKey := types.NamespacedName{Name: namer.CrToSS(crName) + "-" + strconv.Itoa(podOrdinal), Namespace: defaultNamespace}
		pod := &corev1.Pod{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, ssKey, currentSS)).Should(Succeed())
			g.Expect(k8sClient.Get(ctx, podKey, pod)).Should(Succeed())
			g.Expect(len(pod.Status.ContainerStatuses)).Should(Equal(1))
			g.Expect(pod.Status.ContainerStatuses[0].State.Running).ShouldNot(BeNil())
		}, existingClusterTimeout, existingClusterInterval).Should(Succeed())

	}
}
