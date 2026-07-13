/*
Copyright 2026.

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

package binder

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	mayv1alpha1 "github.com/konflux-ci/may/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("RunnerBinder Controller", func() {
	var (
		reconciler *RunnerBinderReconciler
		ns         *corev1.Namespace
	)

	BeforeEach(func(ctx context.Context) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "binder-test-",
			},
		}
		Expect(k8sClient.Create(ctx, ns)).Should(Succeed())

		reconciler = &RunnerBinderReconciler{
			Client: k8sClient,
			Scheme: scheme.Scheme,
		}
	})

	AfterEach(func(ctx context.Context) {
		Expect(k8sClient.Delete(ctx, ns)).Should(Succeed())
	})

	createRunner := func(ctx context.Context, name string, inUseByName, inUseByNamespace string) *mayv1alpha1.Runner {
		r := &mayv1alpha1.Runner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns.Name,
			},
			Spec: mayv1alpha1.RunnerSpec{
				Flavor: "amd64",
				Resources: corev1.ResourceList{
					corev1.ResourceName("amd64"): resource.MustParse("1"),
				},
				InUseBy: &mayv1alpha1.ClaimReference{
					Name:      inUseByName,
					Namespace: inUseByNamespace,
				},
			},
		}
		Expect(k8sClient.Create(ctx, r)).Should(Succeed())
		return r
	}

	createSecret := func(ctx context.Context, name, namespace string, data map[string][]byte) *corev1.Secret {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Data: data,
			Type: corev1.SecretTypeOpaque,
		}
		Expect(k8sClient.Create(ctx, s)).Should(Succeed())
		return s
	}

	reconcileRunner := func(ctx context.Context, r *mayv1alpha1.Runner) (reconcile.Result, error) {
		return reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(r),
		})
	}

	When("Runner does not exist", func() {
		It("should return no error", func(ctx context.Context) {
			By("reconciling a non-existent Runner")
			r := &mayv1alpha1.Runner{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent",
					Namespace: ns.Name,
				},
			}
			Expect(reconcileRunner(ctx, r)).Should(Equal(reconcile.Result{}))
		})
	})

	When("Runner exists but its Secret is missing", func() {
		It("should return an error", func(ctx context.Context) {
			By("creating a Runner with InUseBy set but no Secret and reconciling")
			Expect(reconcileRunner(ctx, createRunner(ctx, "runner-no-secret", "some-claim", ns.Name))).Error().Should(HaveOccurred())
		})
	})

	When("Runner exists and claimer Secret already exists", func() {
		It("should return no error", func(ctx context.Context) {
			By("creating a Runner, its Secret and the claimer Secret")
			r := createRunner(ctx, "runner-with-secret", "existing-claim", ns.Name)
			createSecret(ctx, r.Name, ns.Name, map[string][]byte{"id_rsa": []byte("dummy-key")})
			createSecret(ctx, "existing-claim", ns.Name, map[string][]byte{"otp": []byte("already-exists")})

			By("reconciling the Runner")
			Expect(reconcileRunner(ctx, r)).Should(Equal(reconcile.Result{}))
		})
	})

	When("Runner exists and OTP TLS cert Secret is missing", func() {
		It("should return an error", func(ctx context.Context) {
			By("creating a Runner and its Secret but no OTP TLS cert Secret")
			createRunner(ctx, "runner-no-otp-cert", "missing-claim", ns.Name)
			createSecret(ctx, "runner-no-otp-cert", ns.Name, map[string][]byte{"id_rsa": []byte("dummy-key")})

			By("reconciling the Runner")
			Expect(reconcileRunner(ctx, &mayv1alpha1.Runner{ObjectMeta: metav1.ObjectMeta{Name: "runner-no-otp-cert", Namespace: ns.Name}})).Error().Should(HaveOccurred())
		})
	})
})
