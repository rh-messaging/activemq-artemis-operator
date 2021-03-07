package v2alpha2_test

import (
	brokerv2alpha2 "github.com/artemiscloud/activemq-artemis-operator/pkg/apis/broker/v2alpha2"
	routev1 "github.com/openshift/api/route/v1"
	extv1b1 "k8s.io/api/extensions/v1beta1"

	. "github.com/artemiscloud/activemq-artemis-operator/pkg/controller/broker/v2alpha2/activemqartemis"
	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

//buildReconcileWithFakeClientWithMocks return reconcile with fake client, schemes and mock objects
func buildReconcileWithFakeClientWithMocks(objs []runtime.Object) *ReconcileActiveMQArtemis {

	registerObjs := []runtime.Object{&brokerv2alpha2.ActiveMQArtemis{}, &corev1.Service{}, &appsv1.StatefulSet{}, &appsv1.StatefulSetList{}, &corev1.Pod{}, &routev1.Route{}, &routev1.RouteList{}, &extv1b1.Ingress{}, &extv1b1.IngressList{}, &corev1.PersistentVolumeClaimList{}, &corev1.ServiceList{}}
	registerObjs = append(registerObjs)
	brokerv2alpha2.SchemeBuilder.Register(registerObjs...)
	brokerv2alpha2.SchemeBuilder.Register()

	scheme, err := brokerv2alpha2.SchemeBuilder.Build()
	if err != nil {
		ginkgo.Fail("Unable to build scheme")
	}
	client := fake.NewFakeClientWithScheme(scheme, objs...)

	result := NewReconcileActiveMQArtemis(client, scheme)
	return &result

}
