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
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

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

func CleanResource(res client.Object, name string, namespace string) {
	CleanResourceWithTimeouts(res, name, namespace, timeout, interval)
}

func CleanResourceWithTimeouts(res client.Object, name string, namespace string, cleanTimeout time.Duration, cleanInterval time.Duration) {
	Expect(k8sClient.Delete(ctx, res)).Should(Succeed())
	By("make sure resource is gone")
	Eventually(func(g Gomega) {
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, res)
		g.Expect(errors.IsNotFound(err)).To(BeTrue())
	}, cleanTimeout, cleanInterval).Should(Succeed())
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
	}, timeout, interval*5).Should(Succeed())

	return outPutbuffer.String()
}

func CleanClusterResource(res client.Object, name string, namespace string) {
	CleanResourceWithTimeouts(res, name, namespace, existingClusterTimeout, existingClusterInterval)
}
