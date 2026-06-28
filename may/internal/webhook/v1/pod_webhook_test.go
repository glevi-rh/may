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

package v1

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/konflux-ci/may/pkg/constants"
	"github.com/konflux-ci/may/pkg/pod"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newPod(name string, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
	}
}

var _ = Describe("Pod Webhook", func() {
	var defaulter PodCustomDefaulter

	BeforeEach(func(ctx context.Context) {
		defaulter = PodCustomDefaulter{}
	})

	When("pod has a flavor annotation", func() {
		It("should gate the pod and increment the metric", func(ctx context.Context) {
			By("recording the metric value before defaulting")
			before := testutil.ToFloat64(podsGated)

			By("calling the defaulter")
			p := newPod("test-pod", map[string]string{pod.KueueFlavorLabelPrefix + "amd64": ""})
			Expect(defaulter.Default(ctx, p)).Should(Succeed())

			By("verifying the scheduling gate was added")
			Expect(p.Spec.SchedulingGates).Should(ContainElement(
				corev1.PodSchedulingGate{Name: constants.MayPodSchedulingGate},
			))

			By("verifying the metric was incremented by 1")
			Expect(testutil.ToFloat64(podsGated)).Should(Equal(before + 1))
		})
	})

	When("pod already has the scheduling gate", func() {
		It("should not add a duplicate gate or increment the metric", func(ctx context.Context) {
			By("recording the metric value before defaulting")
			before := testutil.ToFloat64(podsGated)

			By("creating a pod that already has the scheduling gate")
			p := newPod("already-gated-pod", map[string]string{pod.KueueFlavorLabelPrefix + "amd64": ""})
			p.Spec.SchedulingGates = []corev1.PodSchedulingGate{
				{Name: constants.MayPodSchedulingGate},
			}

			By("calling the defaulter")
			Expect(defaulter.Default(ctx, p)).Should(Succeed())

			By("verifying only one scheduling gate exists")
			Expect(p.Spec.SchedulingGates).Should(HaveLen(1))

			By("verifying the metric was not incremented")
			Expect(testutil.ToFloat64(podsGated)).Should(Equal(before))
		})
	})

	DescribeTable("should not gate the pod",
		func(ctx context.Context, annotations map[string]string) {
			By("recording the metric value before defaulting")
			before := testutil.ToFloat64(podsGated)

			By("calling the defaulter")
			p := newPod("test-pod", annotations)
			Expect(defaulter.Default(ctx, p)).Should(Succeed())

			By("verifying no scheduling gate was added")
			Expect(p.Spec.SchedulingGates).Should(BeEmpty())

			By("verifying the metric was not incremented")
			Expect(testutil.ToFloat64(podsGated)).Should(Equal(before))
		},
		Entry("when pod has no flavor annotation",
			map[string]string{"some-other": "annotation"}),
		Entry("when pod has nil annotations",
			nil),
	)
})
