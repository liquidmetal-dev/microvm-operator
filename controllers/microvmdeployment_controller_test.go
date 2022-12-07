package controllers_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/weaveworks-liquidmetal/controller-pkg/types/microvm"
	infrav1 "github.com/weaveworks-liquidmetal/microvm-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestMicrovmDep_Reconcile_MissingObject(t *testing.T) {
	g := NewWithT(t)

	mvmDep := &infrav1.MicrovmDeployment{}
	objects := []runtime.Object{mvmDep}

	client := createFakeClient(g, objects)
	result, err := reconcileMicrovmReplicaSet(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when microvmdeployment doesn't exist should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect no requeue to be requested")
}

func TestMicrovmDep_ReconcileNormal_CreateSucceeds(t *testing.T) {
	g := NewWithT(t)

	// creating a deployment with 2 hosts and 2 microvms per host
	var (
		expectedReplicas      int32 = 2
		expectedReplicaSets   int   = 2
		expectedTotalMicrovms int32 = 4
	)

	mvmD := createMicrovmDeployment(expectedReplicas, expectedReplicaSets)
	objects := []runtime.Object{mvmD}
	client := createFakeClient(g, objects)

	// first reconciliation
	result, err := reconcileMicrovmDeployment(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmdeployment the first time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	reconciled, err := getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvmdeployment should not fail")
	assertMDFinalizer(g, reconciled)

	assertConditionFalse(g, reconciled, infrav1.MicrovmDeploymentReadyCondition, infrav1.MicrovmDeploymentIncompleteReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmDeployment should not be ready yet")
	g.Expect(reconciled.Status.Replicas).To(Equal(int32(0)), "Expected the record to not have been updated yet")
	g.Expect(microvmReplicaSetsCreated(g, client)).To(Equal(expectedReplicaSets-1), "Expected only one replicaset to have been created after one reconciliation")

	// second reconciliation
	ensureMicrovmReplicaSetState(g, client, expectedReplicas, expectedReplicas-1)
	g.Expect(err).NotTo(HaveOccurred(), "reconciling microvmReplicaSet should not error")
	result, err = reconcileMicrovmDeployment(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmdeployment the second time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	reconciled, err = getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvmdeployment should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmDeploymentReadyCondition, infrav1.MicrovmDeploymentIncompleteReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmDeployment should not be ready yet")
	g.Expect(reconciled.Status.Replicas).To(Equal(expectedTotalMicrovms-2), "Expected the record to contain 2 replicas")
	g.Expect(microvmReplicaSetsCreated(g, client)).To(Equal(expectedReplicaSets), "Expected all Microvms to have been created after two reconciliations")

	// final reconciliation
	ensureMicrovmReplicaSetState(g, client, expectedReplicas, expectedReplicas)
	result, err = reconcileMicrovmDeployment(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmdeployment the third time should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect requeue to not be requested after create")

	reconciled, err = getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvmdeployment should not fail")

	assertConditionTrue(g, reconciled, infrav1.MicrovmDeploymentReadyCondition)
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "MicrovmDeployment should be ready now")
	g.Expect(reconciled.Status.Replicas).To(Equal(expectedTotalMicrovms), "Expected the record to contain 4 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(expectedTotalMicrovms), "Expected all replicas to be ready")
	g.Expect(microvmReplicaSetsCreated(g, client)).To(Equal(expectedReplicaSets), "Expected all Microvms to have been created after two reconciliations")
	assertOneSetPerHost(g, reconciled, client)
}

func TestMicrovmDep_ReconcileNormal_UpdateSucceeds(t *testing.T) {
	g := NewWithT(t)

	// updating a replicaset with 2 replicas
	var (
		initialReplicaSetCount int   = 2
		scaledReplicaSetCount  int32 = 1
		expectedReplicas       int32 = 2
		initialReplicaCount    int32 = 4
		scaledReplicaCount     int32 = 2
	)

	mvmD := createMicrovmDeployment(expectedReplicas, initialReplicaSetCount)
	objects := []runtime.Object{mvmD}
	client := createFakeClient(g, objects)

	// create
	g.Expect(reconcileMicrovmDeploymentNTimes(g, client, initialReplicaSetCount+1, expectedReplicas, expectedReplicas)).To(Succeed())

	reconciled, err := getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertMDFinalizer(g, reconciled)
	assertConditionTrue(g, reconciled, infrav1.MicrovmDeploymentReadyCondition)
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "MicrovmDeployment should be ready now")
	g.Expect(reconciled.Status.Replicas).To(Equal(initialReplicaCount), "Expected the record to contain 4 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(initialReplicaCount), "Expected all replicas to be ready")
	g.Expect(microvmReplicaSetsCreated(g, client)).To(Equal(initialReplicaSetCount), "Expected 2 replicasets to exist")

	// update, scale down to 1
	reconciled.Spec.Hosts = []microvm.Host{{Endpoint: "1.2.3.4:9090"}}
	g.Expect(client.Update(context.TODO(), reconciled)).To(Succeed())

	// first reconciliation
	result, err := reconcileMicrovmDeployment(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmdeployment the first time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after update")

	reconciled, err = getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvmdeployment should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmDeploymentReadyCondition, infrav1.MicrovmDeploymentUpdatingReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmDeployment should not be ready")
	g.Expect(reconciled.Status.Replicas).To(Equal(initialReplicaCount), "Expected the record to contain 4 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(initialReplicaCount), "Expected all replicas to be ready")

	// second reconciliation
	result, err = reconcileMicrovmDeployment(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmdeployment the second time should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect requeue to not be requested after reconcile")

	reconciled, err = getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvmdeployment should not fail")

	assertConditionTrue(g, reconciled, infrav1.MicrovmDeploymentReadyCondition)
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "MicrovmDeployment should be ready again")
	g.Expect(reconciled.Status.Replicas).To(Equal(scaledReplicaCount), "Expected the record to contain 2 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(scaledReplicaCount), "Expected all replicas to be ready")
	g.Expect(microvmReplicaSetsCreated(g, client)).To(Equal(int(scaledReplicaSetCount)), "Expected replicasets to have been scaled down after two reconciliations")
}

func TestMicrovmDep_ReconcileDelete_DeleteSucceeds(t *testing.T) {
	g := NewWithT(t)

	// updating a replicaset with 2 replicas
	var (
		initialReplicaSetCount int   = 2
		expectedReplicas       int32 = 2
		initialReplicaCount    int32 = 4
	)

	mvmD := createMicrovmDeployment(expectedReplicas, initialReplicaSetCount)
	objects := []runtime.Object{mvmD}
	client := createFakeClient(g, objects)

	// create
	g.Expect(reconcileMicrovmDeploymentNTimes(g, client, initialReplicaSetCount+1, expectedReplicas, expectedReplicas)).To(Succeed())

	reconciled, err := getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertMDFinalizer(g, reconciled)
	assertConditionTrue(g, reconciled, infrav1.MicrovmDeploymentReadyCondition)
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "MicrovmDeployment should be ready now")
	g.Expect(reconciled.Status.Replicas).To(Equal(initialReplicaCount), "Expected the record to contain 4 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(initialReplicaCount), "Expected all replicas to be ready")
	g.Expect(microvmReplicaSetsCreated(g, client)).To(Equal(initialReplicaSetCount), "Expected 2 replicasets to exist")

	// delete
	g.Expect(client.Delete(context.TODO(), reconciled)).To(Succeed())

	// first reconciliation
	result, err := reconcileMicrovmDeployment(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmdeployment the first time should not error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after update")

	reconciled, err = getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvmdeployment should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmDeploymentReadyCondition, infrav1.MicrovmDeploymentDeletingReason)
	g.Expect(reconciled.Status.Ready).To(BeFalse(), "MicrovmDeployment should not be ready")
	g.Expect(reconciled.Status.Replicas).To(Equal(initialReplicaCount), "Expected the record to contain 4 replicas")
	g.Expect(reconciled.Status.ReadyReplicas).To(Equal(int32(0)), "Expected no replicas to be ready")
	g.Expect(microvmReplicaSetsCreated(g, client)).To(Equal(0), "Expected all replicasets to have been deleted")

	// second reconciliation
	result, err = reconcileMicrovmDeployment(client)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling microvmdeployment the second time should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect requeue to not be requested after reconcile")

	reconciled, err = getMicrovmDeployment(client, testMicrovmDeploymentName, testNamespace)
	g.Expect(err).To(HaveOccurred(), "Getting microvmdeployment should fail")
}
