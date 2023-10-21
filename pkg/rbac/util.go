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
package rbacutil

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

func CreateServiceAccount(name string, namespace string, kubeclientset kubernetes.Interface) (*corev1.ServiceAccount, error) {
	log := ctrl.Log.WithName("rbac")
	getOps := metav1.GetOptions{}
	result, err := kubeclientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, getOps)
	if err == nil {
		log.V(1).Info("Service account already exist", "name", name, "namespace", namespace)
		return result, nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, "Error in finding service account, will create it anyway", "name", name, "namespace", namespace)
	}
	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	log.V(1).Info("Creating service account", "name", name, "namespace", namespace)
	result, err = kubeclientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), serviceAccount, metav1.CreateOptions{})
	if err != nil {
		log.Error(err, "Failed to create service account", "name", name, "namespace", namespace)
		return nil, err
	}
	return result, nil
}

func DeleteServiceAccount(name string, namespace string, kubeclientset kubernetes.Interface) error {
	log := ctrl.Log.WithName("rbac")
	getOps := metav1.GetOptions{}
	_, err := kubeclientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, getOps)
	if err != nil {
		if errors.IsNotFound(err) {
			log.V(1).Info("No such service account")
			return nil
		}
		log.Error(err, "Error in finding service account")
		// will try delete anyway
	}
	deletePeriod := int64(0)
	options := metav1.DeleteOptions{
		GracePeriodSeconds: &deletePeriod,
	}

	if err := kubeclientset.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), name, options); err != nil {
		log.Error(err, "Failed to delete drain service account", "name", name, "namespace", namespace)
	}
	return err
}

func CreateRole(name string, namespace string, rules []rbacv1.PolicyRule, kubeclientset kubernetes.Interface) (*rbacv1.Role, error) {
	log := ctrl.Log.WithName("rbac")
	getOps := metav1.GetOptions{}
	result, err := kubeclientset.RbacV1().Roles(namespace).Get(context.TODO(), name, getOps)
	if err == nil {
		log.V(1).Info("Role already exist", "name", name, "namespace", namespace)
		return result, nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, "Error in finding role, will create it anyway", "name", name, "namespace", namespace)
	}

	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: rules,
	}
	log.V(1).Info("Creating role", "name", name, "namespace", namespace)
	result, err = kubeclientset.RbacV1().Roles(namespace).Create(context.TODO(), role, metav1.CreateOptions{})
	if err != nil {
		log.Error(err, "Failed to create role", "role", name, "namespace", namespace)
		return nil, err
	}
	return result, nil
}

func DeleteRole(name string, namespace string, kubeclientset kubernetes.Interface) error {
	log := ctrl.Log.WithName("rbac")
	getOps := metav1.GetOptions{}
	_, err := kubeclientset.RbacV1().Roles(namespace).Get(context.TODO(), name, getOps)
	if err != nil {
		if errors.IsNotFound(err) {
			log.V(1).Info("No such role", "role", name, "namespace", namespace)
			return nil
		}
		log.Error(err, "Error in role", "role", name, "namespace", namespace)
		// will try delete anyway
	}
	deletePeriod := int64(0)
	options := metav1.DeleteOptions{
		GracePeriodSeconds: &deletePeriod,
	}

	if err := kubeclientset.RbacV1().Roles(namespace).Delete(context.TODO(), name, options); err != nil {
		log.Error(err, "Failed to delete role", "name", name, "namespace", namespace)
	}
	return err
}

func CreateServiceAccountRoleBinding(serviceAccountName string, roleName string, name string, namespace string, kubeclientset kubernetes.Interface) (*rbacv1.RoleBinding, error) {
	log := ctrl.Log.WithName("rbac")
	getOps := metav1.GetOptions{}
	result, err := kubeclientset.RbacV1().RoleBindings(namespace).Get(context.TODO(), name, getOps)
	if err == nil {
		log.V(1).Info("RoleBinding already exist", "name", name, "namespace", namespace)
		return result, nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, "Error in finding RoleBinding, will create it anyway", "name", name, "namespace", namespace)
	}
	roleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: serviceAccountName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     roleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	log.V(1).Info("Creating role binding", "name", name, "namespace", namespace)
	result, err = kubeclientset.RbacV1().RoleBindings(namespace).Create(context.TODO(), roleBinding, metav1.CreateOptions{})
	if err != nil {
		log.Error(err, "Failed to create rolebinding", "name", name, "namespace", namespace)
		return nil, err
	}
	return result, nil
}

func DeleteRoleBinding(name string, namespace string, kubeclientset kubernetes.Interface) error {
	log := ctrl.Log.WithName("rbac")
	getOps := metav1.GetOptions{}
	_, err := kubeclientset.RbacV1().RoleBindings(namespace).Get(context.TODO(), name, getOps)
	if err != nil {
		if errors.IsNotFound(err) {
			log.V(1).Info("No such rolebinding to delete", "name", name, "namespace", namespace)
			return nil
		}
		log.Error(err, "Error in getting rolebinding", "name", name, "namespace", namespace)
		// will try delete anyway
	}
	deletePeriod := int64(0)
	options := metav1.DeleteOptions{
		GracePeriodSeconds: &deletePeriod,
	}

	if err := kubeclientset.RbacV1().RoleBindings(namespace).Delete(context.TODO(), name, options); err != nil {
		log.Error(err, "Failed to delete rolebinding", "name", name, "namespace", namespace)
	}
	return err
}
