/*
Copyright 2025.

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

package claimer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/konflux-ci/may/api/v1alpha1"
	"github.com/konflux-ci/may/pkg/constants"
	"github.com/konflux-ci/may/pkg/pod"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ClaimerController", func() {
	const (
		podName  = "test-pod"
		flavor   = "amd64"
		pipeline = "my-pipeline"
	)

	var (
		reconciler *ClaimerController
		tenantNs   *corev1.Namespace
		regularNs  *corev1.Namespace
	)

	BeforeEach(func() {
		reconciler = &ClaimerController{
			Client: k8sClient,
			Scheme: scheme.Scheme,
		}

		tenantNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "tenant-",
				Labels: map[string]string{
					constants.TenantNamespaceLabelKey: constants.TenantNamespaceLabelValue,
				},
			},
		}
		Expect(k8sClient.Create(ctx, tenantNs)).Should(Succeed())

		regularNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "regular-",
			},
		}
		Expect(k8sClient.Create(ctx, regularNs)).Should(Succeed())
	})

	AfterEach(func() {
		Expect(k8sClient.Delete(ctx, tenantNs)).Should(Succeed())
		Expect(k8sClient.Delete(ctx, regularNs)).Should(Succeed())
	})

	createPod := func(namespace string, annotations, labels map[string]string) *corev1.Pod {
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:        podName,
				Namespace:   namespace,
				Annotations: annotations,
				Labels:      labels,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{Name: "build", Image: "registry.example.com/builder:latest"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, p)).Should(Succeed())
		return p
	}

	reconcilePod := func(namespace string) (reconcile.Result, error) {
		return reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: podName, Namespace: namespace},
		})
	}

	Describe("Reconcile", func() {
		When("a matching pod exists in a tenant namespace", func() {
			It("should create a Claim with the correct flavor and owner reference", func() {
				p := createPod(tenantNs.Name,
					map[string]string{pod.KueueFlavorLabelPrefix + flavor: ""},
					nil,
				)

				result, err := reconcilePod(tenantNs.Name)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(result).Should(Equal(reconcile.Result{}))

				claim := &v1alpha1.Claim{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: tenantNs.Name,
				}, claim)).Should(Succeed())

				Expect(claim.Spec.Flavor).Should(Equal(flavor))
				Expect(claim.Spec.For.Name).Should(Equal(p.Name))
				Expect(claim.Spec.For.UID).Should(Equal(p.UID))

				Expect(claim.OwnerReferences).Should(HaveLen(1))
				Expect(claim.OwnerReferences[0].Name).Should(Equal(p.Name))
				Expect(claim.OwnerReferences[0].UID).Should(Equal(p.UID))
			})
		})

		When("the pod has a pipeline label", func() {
			It("should copy the pipeline label to the Claim", func() {
				createPod(tenantNs.Name,
					map[string]string{pod.KueueFlavorLabelPrefix + flavor: ""},
					map[string]string{pipelineLabelKey: pipeline},
				)

				_, err := reconcilePod(tenantNs.Name)
				Expect(err).ShouldNot(HaveOccurred())

				claim := &v1alpha1.Claim{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: tenantNs.Name,
				}, claim)).Should(Succeed())

				Expect(claim.Labels).Should(HaveKeyWithValue(pipelineLabelKey, pipeline))
			})
		})

		When("the pod has no pipeline label", func() {
			It("should create a Claim without the pipeline label", func() {
				createPod(tenantNs.Name,
					map[string]string{pod.KueueFlavorLabelPrefix + flavor: ""},
					nil,
				)

				_, err := reconcilePod(tenantNs.Name)
				Expect(err).ShouldNot(HaveOccurred())

				claim := &v1alpha1.Claim{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: tenantNs.Name,
				}, claim)).Should(Succeed())

				Expect(claim.Labels).ShouldNot(HaveKey(pipelineLabelKey))
			})
		})

		When("the pod is in a non-tenant namespace", func() {
			It("should not create a Claim", func() {
				createPod(regularNs.Name,
					map[string]string{pod.KueueFlavorLabelPrefix + flavor: ""},
					nil,
				)

				result, err := reconcilePod(regularNs.Name)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(result).Should(Equal(reconcile.Result{}))

				claim := &v1alpha1.Claim{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      podName,
					Namespace: regularNs.Name,
				}, claim)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("not found"))
			})
		})

		When("the pod does not exist", func() {
			It("should return an error", func() {
				_, err := reconcilePod(tenantNs.Name)
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("not found"))
			})
		})

		When("the namespace does not exist", func() {
			It("should return an error", func() {
				createPod(tenantNs.Name,
					map[string]string{pod.KueueFlavorLabelPrefix + flavor: ""},
					nil,
				)

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: podName, Namespace: "nonexistent"},
				})
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring("not found"))
			})
		})

		When("a Claim already exists for the pod", func() {
			It("should not return an error", func() {
				createPod(tenantNs.Name,
					map[string]string{pod.KueueFlavorLabelPrefix + flavor: ""},
					nil,
				)

				_, err := reconcilePod(tenantNs.Name)
				Expect(err).ShouldNot(HaveOccurred())

				_, err = reconcilePod(tenantNs.Name)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})
})
