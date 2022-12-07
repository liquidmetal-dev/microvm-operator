package controllers_test

import (
	"testing"

	. "github.com/onsi/gomega"
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
