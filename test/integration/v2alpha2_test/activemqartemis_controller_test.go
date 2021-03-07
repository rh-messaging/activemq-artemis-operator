package v2alpha2_test

import (
	"fmt"
	brokerv2alpha1 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha2"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/environments"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/services"
	"github.com/artemiscloud/activemq-artemis-operator/pkg/resources/statefulsets"
	routev1 "github.com/openshift/api/route/v1"
	extv1b1 "k8s.io/api/extensions/v1beta1"
	//"github.com/stretchr/testify/assert"
	//"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strconv"
	//"testing"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

var log = logf.Log.WithName("controller_v2alpha2activemqartemis_test")

var _ = ginkgo.Describe("CRD Validation Test", func() {
	ginkgo.It("TestNonWatchedResourceNameNotFound", func() {
		testNonWatchedResourceNameNotFound()
	})

	ginkgo.It("TestNonWatchedResourceNamespaceNotFound", func() {
		testNonWatchedResourceNamespaceNotFound()
	})

	ginkgo.Context("TestActiveMQArtemisController_Reconcile", func() {
		logf.SetLogger(logf.ZapLogger(true))
		objs := []runtime.Object{
			&AMQinstance,
		}
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      AMQinstance.Name,
				Namespace: AMQinstance.Namespace,
			},
		}
		r := buildReconcileWithFakeClientWithMocks(objs)

		res, err := r.Reconcile(request)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(res).Should(gomega.Equal(reconcile.Result{}))

		isOpenshift := false
		if isOpenshift, err = environments.DetectOpenshift(); err != nil {
			log.Error(err, "Failed to get env, will try kubernetes")
		}

		NamespacedName := types.NamespacedName{Name: AMQinstance.Name, Namespace: AMQinstance.Namespace}
		ginkgo.Context("ActiveMQArtemis", func() {
			ginkgo.It("cluster", func() {
				amq := brokerv2alpha1.ActiveMQArtemis{}
				err = r.GetClient().Get(context.TODO(), NamespacedName, &amq)
				gomega.Expect(err).Should(gomega.BeNil())
			})
			StatefulSet := &appsv1.StatefulSet{}
			statefulsetName := types.NamespacedName{Name: statefulsets.NameBuilder.Name(), Namespace: AMQinstance.Namespace}
			err = r.GetClient().Get(context.TODO(), statefulsetName, StatefulSet)
			gomega.Expect(err).Should(gomega.BeNil())

			ginkgo.It("check if the StatefulSet has the correct Replicas's size", func() {
				sfsize := *StatefulSet.Spec.Replicas
				if sfsize != AMQinstance.Spec.DeploymentPlan.Size {
					errMsg := fmt.Sprintf("sfsize size (%d) is not the expected size (%d)", sfsize, AMQinstance.Spec.DeploymentPlan.Size)
					ginkgo.Fail(errMsg)
				}
				gomega.Expect(sfsize).Should(gomega.Equal(AMQinstance.Spec.DeploymentPlan.Size))
			})

			ginkgo.Context("check if the services has been created", func() {
				service := &corev1.Service{}
				ginkgo.It("Headless Service", func() {
					HeadlessServiceName := types.NamespacedName{Name: services.HeadlessNameBuilder.Name(), Namespace: AMQinstance.Namespace}
					err = r.GetClient().Get(context.TODO(), HeadlessServiceName, service)
					gomega.Expect(err).Should(gomega.BeNil())
				})
				ginkgo.It("Ping Service", func() {
					PingServiceName := types.NamespacedName{Name: services.PingNameBuilder.Name(), Namespace: AMQinstance.Namespace}
					err = r.GetClient().Get(context.TODO(), PingServiceName, service)
					gomega.Expect(err).Should(gomega.BeNil())
				})
				var i int32 = 0
				ordinalString := ""
				for ; i < AMQinstance.Spec.DeploymentPlan.Size; i++ {
					ordinalString = strconv.Itoa(int(i))
					for _, acceptor := range AMQinstance.Spec.Acceptors {
						acceptorService := types.NamespacedName{Name: AMQinstance.Name + "-" + acceptor.Name + "-" + ordinalString + "-svc", Namespace: AMQinstance.Namespace}
						err = r.GetClient().Get(context.TODO(), acceptorService, service)
						gomega.Expect(err).Should(gomega.BeNil())
					}
					for _, connector := range AMQinstance.Spec.Connectors {
						connectorService := types.NamespacedName{Name: AMQinstance.Name + "-" + connector.Name + "-" + ordinalString + "-svc", Namespace: AMQinstance.Namespace}
						err = r.GetClient().Get(context.TODO(), connectorService, service)
						gomega.Expect(err).Should(gomega.BeNil())
					}
					wconsjService := types.NamespacedName{Name: AMQinstance.Name + "-wconsj-" + ordinalString + "-svc", Namespace: AMQinstance.Namespace}
					err = r.GetClient().Get(context.TODO(), wconsjService, service)
					gomega.Expect(err).Should(gomega.BeNil())
				}
			})

			var i int32 = 0
			ordinalString := ""
			route := &routev1.Route{}
			ingress := &extv1b1.Ingress{}
			for ; i < AMQinstance.Spec.DeploymentPlan.Size; i++ {
				ordinalString = strconv.Itoa(int(i))
				for _, acceptor := range AMQinstance.Spec.Acceptors {
					acceptorRoute := types.NamespacedName{Name: AMQinstance.Name + "-" + acceptor.Name + "-" + ordinalString + "-svc-rte", Namespace: AMQinstance.Namespace}
					err = r.GetClient().Get(context.TODO(), acceptorRoute, route)
					gomega.Expect(err).Should(gomega.BeNil())
				}
				for _, connector := range AMQinstance.Spec.Connectors {
					connectorRoute := types.NamespacedName{Name: AMQinstance.Name + "-" + connector.Name + "-" + ordinalString + "-svc-rte", Namespace: AMQinstance.Namespace}
					err = r.GetClient().Get(context.TODO(), connectorRoute, route)
					gomega.Expect(err).Should(gomega.BeNil())
				}

				if isOpenshift {
					consoleRoute := types.NamespacedName{Name: AMQinstance.Name + "-wconsj" + "-" + ordinalString + "-svc-rte", Namespace: AMQinstance.Namespace}
					err = r.GetClient().Get(context.TODO(), consoleRoute, route)
					gomega.Expect(err).Should(gomega.BeNil())
				} else {
					consoleIngress := types.NamespacedName{Name: AMQinstance.Name + "-wconsj" + "-" + ordinalString + "-svc-ing", Namespace: AMQinstance.Namespace}
					err = r.GetClient().Get(context.TODO(), consoleIngress, ingress)
					gomega.Expect(err).Should(gomega.BeNil())
				}
			}

			for i := 0; i < int(AMQinstance.Spec.DeploymentPlan.Size); i++ {
				podTemplate.ObjectMeta.Name = fmt.Sprintf("%s-pod-%d", AMQinstance.Name, i)
				err = r.GetClient().Create(context.TODO(), podTemplate.DeepCopy())
				gomega.Expect(err).Should(gomega.BeNil())
			}

			// Reconcile again and check for errors this time.
			res, err = r.Reconcile(request)
			gomega.Expect(err).Should(gomega.BeNil())
			gomega.Expect(res).Should(gomega.Equal(reconcile.Result{}))
		})
	})

	ginkgo.Context("TestActiveMQArtemis_Reconcile_WithoutSpecDefined", func() {
		logf.SetLogger(logf.ZapLogger(true))
		objs := []runtime.Object{
			&AMQInstanceWithoutSpec,
		}
		request := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      AMQInstanceWithoutSpec.Name,
				Namespace: AMQInstanceWithoutSpec.Namespace,
			},
		}
		r := buildReconcileWithFakeClientWithMocks(objs)
		res, err := r.Reconcile(request)
		gomega.Expect(err).Should(gomega.BeNil())
		gomega.Expect(res).Should(gomega.Equal(reconcile.Result{}))
		NamespacedName := types.NamespacedName{Name: AMQInstanceWithoutSpec.Name, Namespace: AMQInstanceWithoutSpec.Namespace}

		ginkgo.Context("ActiveMQArtemis", func() {
			ginkgo.It("cluster", func() {
				amq := brokerv2alpha1.ActiveMQArtemis{}
				err = r.GetClient().Get(context.TODO(), NamespacedName, &amq)
				gomega.Expect(err).Should(gomega.BeNil())
			})
			StatefulSet := &appsv1.StatefulSet{}
			ginkgo.It("check StatefulSet", func() {
				statefulsetName := types.NamespacedName{Name: statefulsets.NameBuilder.Name(), Namespace: AMQinstance.Namespace}
				err = r.GetClient().Get(context.TODO(), statefulsetName, StatefulSet)
				gomega.Expect(err).Should(gomega.HaveOccurred())
				msg := "not found"
				gomega.Expect(err.Error()).Should(gomega.ContainSubstring(msg))
			})
		})
	})
})

func testNonWatchedResourceNameNotFound() {
	logf.SetLogger(logf.ZapLogger(true))
	objs := []runtime.Object{
		&AMQinstance,
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "doesn't exist",
			Namespace: AMQinstance.Namespace,
		},
	}
	r := buildReconcileWithFakeClientWithMocks(objs)
	result, err := r.Reconcile(request)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(result).To(gomega.Equal(reconcile.Result{}))

}

func testNonWatchedResourceNamespaceNotFound() {
	logf.SetLogger(logf.ZapLogger(true))

	objs := []runtime.Object{
		&AMQinstance,
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      AMQinstance.Name,
			Namespace: "doesn't exist",
		},
	}
	r := buildReconcileWithFakeClientWithMocks(objs)
	result, err := r.Reconcile(request)
	gomega.Expect(err).To(gomega.BeNil())
	gomega.Expect(result).To(gomega.Equal(reconcile.Result{}))
}
