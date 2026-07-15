/*
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
// +kubebuilder:docs-gen:collapse=Apache License
package controllers

import (
	"context"
	"testing"

	brokerv1beta1 "github.com/arkmq-org/arkmq-org-broker-operator/v2/api/v1beta1"
	v1beta2 "github.com/arkmq-org/arkmq-org-broker-operator/v2/api/v1beta2"
	"github.com/arkmq-org/arkmq-org-broker-operator/v2/pkg/utils/common"
	"github.com/arkmq-org/arkmq-org-broker-operator/v2/pkg/utils/selectors"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestErrOnNotFoundSecret(t *testing.T) {

	cr := &v1beta2.Broker{
		ObjectMeta: v1.ObjectMeta{Name: "a"},
		Spec:       v1beta2.BrokerSpec{},
	}

	namer := MakeNamersForBroker(cr)

	r := NewBrokerReconciler(&NillCluster{}, ctrl.Log, isOpenshift)
	ri := NewBrokerReconcilerImpl(cr, r)

	var times = 0
	interceptorFuncs := interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			times++
			return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
		},
	}

	common.SetOperatorNameSpace("test")
	t.Cleanup(common.UnsetOperatorNameSpace)

	client := fake.NewClientBuilder().WithInterceptorFuncs(interceptorFuncs).Build()

	error := ri.Process(cr, *namer, client, nil)

	assert.NotNil(t, error)
	assert.ErrorContains(t, error, "not found")
}

func TestValidateRestrictedNeedsSecret(t *testing.T) {

	cr := &v1beta2.Broker{
		ObjectMeta: v1.ObjectMeta{Name: "a"},
		Spec:       v1beta2.BrokerSpec{},
	}

	r := NewBrokerReconciler(&NillCluster{}, ctrl.Log, isOpenshift)
	ri := NewBrokerReconcilerImpl(cr, r)

	fakeSecrets := map[string]client.Object{}
	interceptorFuncs := interceptor.Funcs{
		Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if o, found := fakeSecrets[key.Name]; found {
				obj.SetName(o.GetName())
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
		},
	}

	common.SetOperatorNameSpace("test")
	t.Cleanup(common.UnsetOperatorNameSpace)

	client := fake.NewClientBuilder().WithInterceptorFuncs(interceptorFuncs).Build()

	valid, retry := ri.validate(cr, client)

	assert.False(t, valid)
	assert.True(t, retry)

	assert.True(t, meta.IsStatusConditionFalse(cr.Status.Conditions, brokerv1beta1.ValidConditionType))

	condition := meta.FindStatusCondition(cr.Status.Conditions, brokerv1beta1.ValidConditionType)
	assert.Equal(t, condition.Reason, brokerv1beta1.ValidConditionMissingResourcesReason)
	assert.Contains(t, condition.Message, "failed to get secret")
	assert.Contains(t, condition.Message, common.DefaultOperatorCertSecretName)

	fakeSecrets[common.DefaultOperatorCertSecretName] = &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: common.DefaultOperatorCertSecretName},
	}

	valid, retry = ri.validate(cr, client)

	assert.False(t, valid)
	assert.True(t, retry)
	assert.True(t, meta.IsStatusConditionFalse(cr.Status.Conditions, brokerv1beta1.ValidConditionType))
	condition = meta.FindStatusCondition(cr.Status.Conditions, brokerv1beta1.ValidConditionType)
	assert.Equal(t, condition.Reason, brokerv1beta1.ValidConditionMissingResourcesReason)
	assert.Contains(t, condition.Message, "failed to get secret")
	assert.Contains(t, condition.Message, common.DefaultOperatorCASecretName)

	fakeSecrets[common.DefaultOperatorCASecretName] = &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: common.DefaultOperatorCASecretName},
	}
	valid, retry = ri.validate(cr, client)

	assert.False(t, valid)
	assert.True(t, retry)
	assert.True(t, meta.IsStatusConditionFalse(cr.Status.Conditions, brokerv1beta1.ValidConditionType))
	condition = meta.FindStatusCondition(cr.Status.Conditions, brokerv1beta1.ValidConditionType)
	assert.Equal(t, condition.Reason, brokerv1beta1.ValidConditionMissingResourcesReason)
	assert.Contains(t, condition.Message, "failed to get secret")
	assert.Contains(t, condition.Message, common.DefaultOperandCertSecretName)

	fakeSecrets[common.DefaultOperandCertSecretName] = &corev1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: common.DefaultOperandCertSecretName},
	}
	valid, retry = ri.validate(cr, client)

	assert.True(t, valid)
	assert.False(t, retry)
	assert.True(t, meta.IsStatusConditionTrue(cr.Status.Conditions, brokerv1beta1.ValidConditionType))
}

func TestMakeNamersForBrokerUsesBrokerTrackingLabel(t *testing.T) {
	cr := &v1beta2.Broker{
		ObjectMeta: v1.ObjectMeta{Name: "my-broker"},
	}

	namer := MakeNamersForBroker(cr)
	labels := namer.LabelBuilder.Labels()

	assert.Equal(t, "my-broker", labels[selectors.LabelBrokerKey])
	assert.Equal(t, "my-broker-app", labels[selectors.LabelAppKey])
	_, hasActiveMQArtemis := labels[selectors.LabelActiveMQArtemisKey]
	assert.False(t, hasActiveMQArtemis)

	defaultLabels := GetDefaultLabelsForBroker(cr)
	assert.Equal(t, labels, defaultLabels)
}

func TestValidateReservedLabelsForBroker(t *testing.T) {
	t.Run("rejects Broker reserved key in Spec.Labels", func(t *testing.T) {
		cr := &v1beta2.Broker{
			Spec: v1beta2.BrokerSpec{
				Labels: map[string]string{selectors.LabelBrokerKey: "x"},
			},
		}
		condition := validateReservedLabelsForBroker(cr)
		assert.NotNil(t, condition)
		assert.Equal(t, v1beta2.ValidConditionFailedReservedLabelReason, condition.Reason)
		assert.Contains(t, condition.Message, "Spec.Labels")
	})

	t.Run("rejects application reserved key in Spec.Labels", func(t *testing.T) {
		cr := &v1beta2.Broker{
			Spec: v1beta2.BrokerSpec{
				Labels: map[string]string{selectors.LabelAppKey: "x"},
			},
		}
		condition := validateReservedLabelsForBroker(cr)
		assert.NotNil(t, condition)
		assert.Equal(t, v1beta2.ValidConditionFailedReservedLabelReason, condition.Reason)
	})

	t.Run("allows custom labels", func(t *testing.T) {
		cr := &v1beta2.Broker{
			Spec: v1beta2.BrokerSpec{
				Labels: map[string]string{"team": "messaging"},
			},
		}
		assert.Nil(t, validateReservedLabelsForBroker(cr))
	})

	t.Run("allows ActiveMQArtemis key on Broker CR", func(t *testing.T) {
		cr := &v1beta2.Broker{
			Spec: v1beta2.BrokerSpec{
				Labels: map[string]string{selectors.LabelActiveMQArtemisKey: "x"},
			},
		}
		assert.Nil(t, validateReservedLabelsForBroker(cr))
	})

	t.Run("rejects Broker reserved key in ResourceTemplates", func(t *testing.T) {
		cr := &v1beta2.Broker{
			Spec: v1beta2.BrokerSpec{
				ResourceTemplates: []v1beta2.ResourceTemplate{
					{Labels: map[string]string{selectors.LabelBrokerKey: "x"}},
				},
			},
		}
		condition := validateReservedLabelsForBroker(cr)
		assert.NotNil(t, condition)
		assert.Equal(t, v1beta2.ValidConditionFailedReservedLabelReason, condition.Reason)
		assert.Contains(t, condition.Message, "Spec.ResourceTemplates[0].Labels")
	})
}
