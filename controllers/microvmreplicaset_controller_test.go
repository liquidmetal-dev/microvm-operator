package controllers_test

import (
	"context"
	"testing"

	infrav1 "github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func TestMicrovmRS_Reconcile_MissingObject(t *testing.T) {
	g := NewWithT(t)

	mvmRS := &infrav1.MicrovmReplicaSet{}
	objects := []runtime.Object{mvmRS}

	client := createFakeClient(g, objects)
	result, err := reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when microvmreplicaset doesn't exist should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect no requeue to be requested")
}

func TestMicrovmRS_ReconcileNormal_CreateSucceeds(t *testing.T) {
	g := NewWithT(t)

	// creating a replicaset with 2 replicas
	var expectedReplicas int32 = 2
	mvmRS := createMicrovmReplicaSet(expectedReplicas)
	objects := []runtime.Object{mvmRS}
	client := createFakeClient(g, objects)

	// first reconciliation
	result, err := reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmreplicaset the first time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	reconciled, err := getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")
	assertMRSFinalizer(g, reconciled)

	assertConditionFalse(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition, infrav1.MicrovmReplicaSetIncompleteReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmReplicaSet should not be ready yet")
	g.Expect(reconciled.Status.Replicas).To(Equal(int32(0)), "Expected the record to not have been updated yet")
	g.Expect(microvmsCreated(g, client)).To(Equal(expectedReplicas-1), "Expected only one Microvm to have been created after one reconciliation")

	// second reconciliation
	result, err = reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmreplicaset the second time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	reconciled, err = getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition, infrav1.MicrovmReplicaSetIncompleteReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmReplicaSet should not be ready yet")
	g.Expect(reconciled.Status.Replicas).To(Equal(expectedReplicas-1), "Expected the record to contain 1 replica")
	g.Expect(microvmsCreated(g, client)).To(Equal(expectedReplicas), "Expected all Microvms to have been created after two reconciliations")

	// final reconciliation
	ensureMicrovmState(g, client)
	result, err = reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmreplicaset the third time should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect requeue to be not requested after create")

	reconciled, err = getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionTrue(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition)
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "MicrovmReplicaSet should be ready now")
	g.Expect(reconciled.Status.Replicas).To(Equal(expectedReplicas), "Expected the record to contain 2 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(expectedReplicas), "Expected all replicas to be ready")
}

func TestMicrovmRS_ReconcileNormal_UpdateSucceeds(t *testing.T) {
	g := NewWithT(t)

	// updating a replicaset with 2 replicas
	var (
		initialReplicaCount int32 = 2
		scaledReplicaCount  int32 = 1
	)

	mvmRS := createMicrovmReplicaSet(initialReplicaCount)
	objects := []runtime.Object{mvmRS}
	client := createFakeClient(g, objects)

	// create
	g.Expect(reconcileMicrovmReplicaSetNTimes(g, client, initialReplicaCount+1)).To(Succeed())

	reconciled, err := getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertMRSFinalizer(g, reconciled)
	assertConditionTrue(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition)
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "MicrovmReplicaSet should be ready now")
	g.Expect(reconciled.Status.Replicas).To(Equal(initialReplicaCount), "Expected the record to contain 2 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(initialReplicaCount), "Expected all replicas to be ready")

	// update, scale down to 1
	reconciled.Spec.Replicas = pointer.Int32(scaledReplicaCount)
	g.Expect(client.Update(context.TODO(), reconciled)).To(Succeed())

	// first reconciliation
	result, err := reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmreplicaset the first time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after update")

	reconciled, err = getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition, infrav1.MicrovmReplicaSetUpdatingReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmReplicaSet should not be ready")
	g.Expect(reconciled.Status.Replicas).To(Equal(initialReplicaCount), "Expected the record to contain 2 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(initialReplicaCount), "Expected all replicas to be ready")

	// second reconciliation
	result, err = reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmreplicaset the second time should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect requeue to not be requested after reconcile")

	reconciled, err = getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionTrue(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition)
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "MicrovmReplicaSet should be ready")
	g.Expect(reconciled.Status.Replicas).To(Equal(scaledReplicaCount), "Expected the record to contain 1 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(scaledReplicaCount), "Expected all replicas to be ready")
	g.Expect(microvmsCreated(g, client)).To(Equal(scaledReplicaCount), "Expected Microvms to have been scaled down after two reconciliations")
}

func TestMicrovmRS_ReconcileDelete_DeleteSucceeds(t *testing.T) {
	g := NewWithT(t)

	// deleting a replicaset with 2 replicas
	var initialReplicaCount int32 = 2

	mvmRS := createMicrovmReplicaSet(initialReplicaCount)
	objects := []runtime.Object{mvmRS}
	client := createFakeClient(g, objects)

	// create
	g.Expect(reconcileMicrovmReplicaSetNTimes(g, client, initialReplicaCount+1)).To(Succeed())

	reconciled, err := getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertMRSFinalizer(g, reconciled)
	assertConditionTrue(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition)
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "MicrovmReplicaSet should be ready now")
	g.Expect(reconciled.Status.Replicas).To(Equal(initialReplicaCount), "Expected the record to contain 2 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(initialReplicaCount), "Expected all replicas to be ready")

	// delete
	g.Expect(client.Delete(context.TODO(), reconciled)).To(Succeed())

	// first reconciliation
	result, err := reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmreplicaset the first time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after update")

	reconciled, err = getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition, infrav1.MicrovmReplicaSetDeletingReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmReplicaSet should not be ready")
	g.Expect(reconciled.Status.Replicas).To(Equal(initialReplicaCount), "Expected the record to contain 2 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(int32(0)), "Expected no replicas to be ready")

	// second reconciliation
	result, err = reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmreplicaset the second time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after reconcile")

	reconciled, err = getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmReplicaSetReadyCondition, infrav1.MicrovmReplicaSetDeletingReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmReplicaSet should not be ready")
	g.Expect(reconciled.Status.Replicas).To(Equal(int32(0)), "Expected the record to contain 0 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(int32(0)), "Expected all no replicas to be ready")
	g.Expect(microvmsCreated(g, client)).To(Equal(int32(0)), "Expected Microvms to have been deleted after two reconciliations")

	// third reconciliation
	result, err = reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmreplicaset the third time should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect requeue to not be requested after reconcile")

	reconciled, err = getMicrovmReplicaSet(client, testMicrovmReplicaSetName, testNamespace)
	g.Expect(err).To(HaveOccurred(), "Getting microvmreplicaset should fail")
}
