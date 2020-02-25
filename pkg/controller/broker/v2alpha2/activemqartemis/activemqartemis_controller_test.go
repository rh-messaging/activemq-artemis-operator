package v2alpha2activemqartemis

import (
	"fmt"
	routev1 "github.com/openshift/api/route/v1"
	brokerv2alpha1 "github.com/rh-messaging/activemq-artemis-operator/pkg/apis/broker/v2alpha2"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/services"
	"github.com/rh-messaging/activemq-artemis-operator/pkg/resources/statefulsets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strconv"
	"testing"
)

func TestNonWatchedResourceNameNotFound(t *testing.T) {
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
	r := buildReconcileWithFakeClientWithMocks(objs, t)
	result, err := r.Reconcile(request)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result)

}
func TestNonWatchedResourceNamespaceNotFound(t *testing.T) {
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
	r := buildReconcileWithFakeClientWithMocks(objs, t)
	result, err := r.Reconcile(request)
	assert.NoError(t, err)
	assert.Equal(t, reconcile.Result{}, result)

}

func TestActiveMQArtemisController_Reconcile(t *testing.T) {
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
	r := buildReconcileWithFakeClientWithMocks(objs, t)

	res, err := r.Reconcile(request)
	assert.NoError(t, err, "reconcile Error ")
	assert.Equal(t, reconcile.Result{}, res)
	NamespacedName := types.NamespacedName{Name: AMQinstance.Name, Namespace: AMQinstance.Namespace}
	t.Run("ActiveMQArtemis", func(t *testing.T) {
		t.Run("cluster", func(t *testing.T) {
			amq := brokerv2alpha1.ActiveMQArtemis{}
			err = r.client.Get(context.TODO(), NamespacedName, &amq)
			require.NoError(t, err)
		})
		StatefulSet := &appsv1.StatefulSet{}
		t.Run("check if the StatefulSet has been created", func(t *testing.T) {
			statefulsetName := types.NamespacedName{Name: statefulsets.NameBuilder.Name(), Namespace: AMQinstance.Namespace}
			err = r.client.Get(context.TODO(), statefulsetName, StatefulSet)
			require.NoError(t, err)

		})
		t.Run("check if the StatefulSet has the correct Replicas's size", func(t *testing.T) {
			sfsize := *StatefulSet.Spec.Replicas
			if sfsize != AMQinstance.Spec.DeploymentPlan.Size {
				t.Errorf("sfsize size (%d) is not the expected size (%d)", sfsize, AMQinstance.Spec.DeploymentPlan.Size)
			}
			assert.Equal(t, sfsize, AMQinstance.Spec.DeploymentPlan.Size)
		})

		t.Run("check if the services has been created", func(t *testing.T) {
			service := &corev1.Service{}
			t.Run("Headless Service", func(t *testing.T) {
				HeadlessServiceName := types.NamespacedName{Name: services.HeadlessNameBuilder.Name(), Namespace: AMQinstance.Namespace}
				err = r.client.Get(context.TODO(), HeadlessServiceName, service)
				require.NoError(t, err)
			})
			t.Run("Ping Service", func(t *testing.T) {
				PingServiceName := types.NamespacedName{Name: services.PingNameBuilder.Name(), Namespace: AMQinstance.Namespace}
				err = r.client.Get(context.TODO(), PingServiceName, service)
				require.NoError(t, err)
			})
			var i int32 = 0
			ordinalString := ""
			for ; i < AMQinstance.Spec.DeploymentPlan.Size; i++ {
				t.Run("CR Service", func(t *testing.T) {
					ordinalString = strconv.Itoa(int(i))
					t.Run("acceptor Service "+ordinalString, func(t *testing.T) {
						for _, acceptor := range AMQinstance.Spec.Acceptors {
							acceptorService := types.NamespacedName{Name: AMQinstance.Name + "-" + acceptor.Name + "-" + ordinalString + "-svc", Namespace: AMQinstance.Namespace}
							err = r.client.Get(context.TODO(), acceptorService, service)
							require.NoError(t, err)
						}
					})
					t.Run("connector Service "+ordinalString, func(t *testing.T) {
						for _, connector := range AMQinstance.Spec.Connectors {
							connectorService := types.NamespacedName{Name: AMQinstance.Name + "-" + connector.Name + "-" + ordinalString + "-svc", Namespace: AMQinstance.Namespace}
							err = r.client.Get(context.TODO(), connectorService, service)
							require.NoError(t, err)
						}
					})
					t.Run("wconsj Service "+ordinalString, func(t *testing.T) {
						wconsjService := types.NamespacedName{Name: AMQinstance.Name + "-wconsj-" + ordinalString + "-svc", Namespace: AMQinstance.Namespace}
						err = r.client.Get(context.TODO(), wconsjService, service)
						require.NoError(t, err)
					})
				})
			}
		})

		t.Run("check if the Routes has been created", func(t *testing.T) {
			var i int32 = 0
			ordinalString := ""
			route := &routev1.Route{}
			for ; i < AMQinstance.Spec.DeploymentPlan.Size; i++ {
				t.Run("CR Route", func(t *testing.T) {
					ordinalString = strconv.Itoa(int(i))
					t.Run("acceptor Route "+ordinalString, func(t *testing.T) {
						for _, acceptor := range AMQinstance.Spec.Acceptors {
							acceptorRoute := types.NamespacedName{Name: AMQinstance.Name + "-" + acceptor.Name + "-" + ordinalString + "-svc-rte", Namespace: AMQinstance.Namespace}
							err = r.client.Get(context.TODO(), acceptorRoute, route)
							require.NoError(t, err)
						}
					})
					t.Run("connector Route "+ordinalString, func(t *testing.T) {
						for _, connector := range AMQinstance.Spec.Connectors {
							connectorRoute := types.NamespacedName{Name: AMQinstance.Name + "-" + connector.Name + "-" + ordinalString + "-svc-rte", Namespace: AMQinstance.Namespace}
							err = r.client.Get(context.TODO(), connectorRoute, route)
							require.NoError(t, err)
						}
					})
					t.Run("console Route "+ordinalString, func(t *testing.T) {
						consoleRoute := types.NamespacedName{Name: AMQinstance.Name + "-wconsj" + "-" + ordinalString + "-svc-rte", Namespace: AMQinstance.Namespace}
						err = r.client.Get(context.TODO(), consoleRoute, route)
						require.NoError(t, err)
					})
				})
			}
		})

		t.Run("check for pod creation", func(t *testing.T) {
			t.Run("create pod", func(t *testing.T) {
				for i := 0; i < int(AMQinstance.Spec.DeploymentPlan.Size); i++ {
					podTemplate.ObjectMeta.Name = fmt.Sprintf("%s-pod-%d", AMQinstance.Name, i)
					err = r.client.Create(context.TODO(), podTemplate.DeepCopy())
					assert.NoError(t, err)
				}
			})

			t.Run("Reconcile again", func(t *testing.T) {
				// Reconcile again and check for errors this time.
				res, err = r.Reconcile(request)
				require.NoError(t, err)
				require.Equal(t, res, reconcile.Result{})
			})
		})

	})

}

func TestActiveMQArtemis_Reconcile_WithoutSpecDefined(t *testing.T) {

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
	r := buildReconcileWithFakeClientWithMocks(objs, t)
	res, err := r.Reconcile(request)
	assert.NoError(t, err, "reconcile Error ")
	assert.Equal(t, reconcile.Result{}, res)
	NamespacedName := types.NamespacedName{Name: AMQInstanceWithoutSpec.Name, Namespace: AMQInstanceWithoutSpec.Namespace}

	t.Run("ActiveMQArtemis", func(t *testing.T) {
		t.Run("cluster", func(t *testing.T) {
			amq := brokerv2alpha1.ActiveMQArtemis{}
			err = r.client.Get(context.TODO(), NamespacedName, &amq)
			require.NoError(t, err)
		})
		StatefulSet := &appsv1.StatefulSet{}
		t.Run("check StatefulSet", func(t *testing.T) {
			statefulsetName := types.NamespacedName{Name: statefulsets.NameBuilder.Name(), Namespace: AMQinstance.Namespace}
			err = r.client.Get(context.TODO(), statefulsetName, StatefulSet)
			require.Error(t, err)
			msg := "not found"
			assert.Contains(t, err.Error(), msg)

		})

	})
}
