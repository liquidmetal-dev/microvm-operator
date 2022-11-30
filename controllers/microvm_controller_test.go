package controllers_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/weaveworks-liquidmetal/controller-pkg/types/microvm"
	flintlocktypes "github.com/weaveworks-liquidmetal/flintlock/api/types"
	infrav1 "github.com/weaveworks-liquidmetal/microvm-operator/api/v1alpha1"
	"github.com/weaveworks-liquidmetal/microvm-operator/controllers/fakes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestMicrovm_Reconcile_MissingObject(t *testing.T) {
	g := NewWithT(t)

	mvm := infrav1.Microvm{}

	client := createFakeClient(g, asRuntimeObject(&mvm))
	result, err := reconcileMicrovm(client, nil)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when microvm doesn't exist should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect no requeue to be requested")
}

func TestMicrovm_Reconcile_MissingHostEndpoint(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()
	mvm.Spec.Host = microvm.Host{}

	client := createFakeClient(g, asRuntimeObject(mvm))
	result, err := reconcileMicrovm(client, nil)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when microvm does not have an endpoint set should not error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect no requeue to be requested")
}

func TestMicrovm_ReconcileNormal_ServiceGetError(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()

	fakeAPIClient := fakes.FakeClient{}
	fakeAPIClient.GetMicroVMReturns(nil, errors.New("something terrible happened"))

	client := createFakeClient(g, asRuntimeObject(mvm))
	_, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).To(HaveOccurred(), "Reconciling when microvm service 'Get' errors should return error")
}

func TestMicrovm_ReconcileNormal_VMExistsAndRunning(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()

	fakeAPIClient := fakes.FakeClient{}
	withExistingMicrovm(&fakeAPIClient, flintlocktypes.MicroVMStatus_CREATED)

	client := createFakeClient(g, asRuntimeObject(mvm))
	result, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when microvm service exists should not return error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expect no requeue to be requested")

	reconciled, err := getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")
	assertMicrovmReconciled(g, reconciled)
}

func TestMicrovm_ReconcileNormal_VMExistsAndPending(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()

	fakeAPIClient := fakes.FakeClient{}
	withExistingMicrovm(&fakeAPIClient, flintlocktypes.MicroVMStatus_PENDING)

	client := createFakeClient(g, asRuntimeObject(mvm))
	result, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when microvm service exists and state pending should not return error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect a requeue to be requested")

	reconciled, err := getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmReadyCondition, infrav1.MicrovmPendingReason)
	assertVMState(g, reconciled, microvm.VMStatePending)
	assertFinalizer(g, reconciled)
}

func TestMicrovm_ReconcileNormal_VMExistsButFailed(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()

	fakeAPIClient := fakes.FakeClient{}
	withExistingMicrovm(&fakeAPIClient, flintlocktypes.MicroVMStatus_FAILED)

	client := createFakeClient(g, asRuntimeObject(mvm))
	_, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).To(HaveOccurred(), "Reconciling when microvm service exists and state failed should return an error")

	reconciled, err := getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmReadyCondition, infrav1.MicrovmProvisionFailedReason)
	assertVMState(g, reconciled, microvm.VMStateFailed)
	assertFinalizer(g, reconciled)
}

func TestMicrovm_ReconcileNormal_VMExistsButUnknownState(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()

	fakeAPIClient := fakes.FakeClient{}
	withExistingMicrovm(&fakeAPIClient, flintlocktypes.MicroVMStatus_MicroVMState(42))

	client := createFakeClient(g, asRuntimeObject(mvm))
	_, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).To(HaveOccurred(), "Reconciling when microvm service exists and state is unknown should return an error")

	reconciled, err := getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmReadyCondition, infrav1.MicrovmUnknownStateReason)
	assertVMState(g, reconciled, microvm.VMStateUnknown)
	assertFinalizer(g, reconciled)
}

func TestMicrovm_ReconcileNormal_NoVmCreateSucceeds(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()
	mvm.Spec.ProviderID = nil

	fakeAPIClient := fakes.FakeClient{}
	withMissingMicrovm(&fakeAPIClient)
	withCreateMicrovmSuccess(&fakeAPIClient)

	client := createFakeClient(g, asRuntimeObject(mvm))
	result, err := reconcileMicrovm(client, &fakeAPIClient)

	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when creating microvm should not return error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	_, createReq, _ := fakeAPIClient.CreateMicroVMArgsForCall(0)
	g.Expect(createReq.Microvm).ToNot(BeNil())

	reconciled, err := getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	expectedProviderID := fmt.Sprintf("microvm://127.0.0.1:9090/%s", testMicrovmUID)
	g.Expect(reconciled.Spec.ProviderID).To(Equal(pointer.String(expectedProviderID)))

	assertConditionFalse(g, reconciled, infrav1.MicrovmReadyCondition, infrav1.MicrovmPendingReason)
	assertVMState(g, reconciled, microvm.VMStatePending)
	assertFinalizer(g, reconciled)
}

func TestMicrovm_ReconcileNormal_NoVmCreateWithUserdataSucceeds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	mvm := createMicrovm()
	mvm.Spec.ProviderID = nil
	mvm.Spec.UserData = pointer.String(testBootstrapData)

	fakeAPIClient := fakes.FakeClient{}
	withMissingMicrovm(&fakeAPIClient)
	withCreateMicrovmSuccess(&fakeAPIClient)

	client := createFakeClient(g, asRuntimeObject(mvm))
	result, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when creating microvm should not return error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	_, createReq, _ := fakeAPIClient.CreateMicroVMArgsForCall(0)
	g.Expect(createReq.Microvm).ToNot(BeNil())
	g.Expect(createReq.Microvm.Metadata).To(HaveLen(3))
	g.Expect(createReq.Microvm.Metadata).To(HaveKeyWithValue("user-data", testBootstrapData))
}

func TestMicrovm_ReconcileNormal_NoVmCreateWithLabelsSucceeds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	expectedLabels := map[string]string{
		"label": "one",
	}

	mvm := createMicrovm()
	mvm.Spec.ProviderID = nil
	mvm.Spec.Labels = expectedLabels

	fakeAPIClient := fakes.FakeClient{}
	withMissingMicrovm(&fakeAPIClient)
	withCreateMicrovmSuccess(&fakeAPIClient)

	client := createFakeClient(g, asRuntimeObject(mvm))
	result, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when creating microvm should not return error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	_, createReq, _ := fakeAPIClient.CreateMicroVMArgsForCall(0)
	g.Expect(createReq.Microvm).ToNot(BeNil())
	g.Expect(createReq.Microvm.Labels).To(HaveLen(1))
	g.Expect(createReq.Microvm.Labels).To(Equal(expectedLabels))
}

func TestMicrovm_ReconcileNormal_NoVmCreateWithSSHSucceeds(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	expectedKeys := []microvm.SSHPublicKey{{
		AuthorizedKeys: []string{"SSH"},
		User:           "root",
	}, {
		AuthorizedKeys: []string{"SSH"},
		User:           "ubuntu",
	}}

	mvm := createMicrovm()
	mvm.Spec.ProviderID = nil
	mvm.Spec.SSHPublicKeys = expectedKeys

	fakeAPIClient := fakes.FakeClient{}
	withMissingMicrovm(&fakeAPIClient)
	withCreateMicrovmSuccess(&fakeAPIClient)

	client := createFakeClient(g, asRuntimeObject(mvm))
	result, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when creating microvm should not return error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	_, createReq, _ := fakeAPIClient.CreateMicroVMArgsForCall(0)
	g.Expect(createReq.Microvm).ToNot(BeNil())
	// g.Expect(createReq.Microvm.Labels).To(HaveLen(1))
	g.Expect(createReq.Microvm.Metadata).To(HaveLen(3))

	// expectedBootstrapData := base64.StdEncoding.EncodeToString([]byte(testbootStrapData))
	// g.Expect(createReq.Microvm.Metadata).To(HaveKeyWithValue("user-data", expectedBootstrapData))

	g.Expect(createReq.Microvm.Metadata).To(HaveKey("vendor-data"), "expect cloud-init vendor-data to be created")
	assertVendorData(g, createReq.Microvm.Metadata["vendor-data"], expectedKeys)
}

func TestMicrovm_ReconcileNormal_NoVmCreateWithAdditionalReconcileSucceeds(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()
	mvm.Spec.ProviderID = nil

	fakeAPIClient := fakes.FakeClient{}
	withMissingMicrovm(&fakeAPIClient)
	withCreateMicrovmSuccess(&fakeAPIClient)

	client := createFakeClient(g, asRuntimeObject(mvm))
	result, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when creating microvm should not return error")
	g.Expect(result.IsZero()).To(BeFalse(), "Expect requeue to be requested after create")

	withExistingMicrovm(&fakeAPIClient, flintlocktypes.MicroVMStatus_CREATED)
	_, err = reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling should not return an error")

	reconciled, err := getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")
	assertMicrovmReconciled(g, reconciled)
}

func TestMicrovm_ReconcileDelete_Succeeds(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()
	mvm.DeletionTimestamp = &metav1.Time{
		Time: time.Now(),
	}
	mvm.Spec.ProviderID = pointer.String(fmt.Sprintf("microvm://127.0.0.1:9090/%s", testMicrovmUID))
	mvm.Finalizers = []string{infrav1.MvmFinalizer}

	fakeAPIClient := fakes.FakeClient{}
	withExistingMicrovm(&fakeAPIClient, flintlocktypes.MicroVMStatus_CREATED)

	client := createFakeClient(g, asRuntimeObject(mvm))

	result, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when deleting microvm should not return error")
	g.Expect(result.Requeue).To(BeFalse())
	g.Expect(result.RequeueAfter).To(BeNumerically(">", time.Duration(0)))

	g.Expect(fakeAPIClient.DeleteMicroVMCallCount()).To(Equal(1))
	_, deleteReq, _ := fakeAPIClient.DeleteMicroVMArgsForCall(0)
	g.Expect(deleteReq.Uid).To(Equal(testMicrovmUID))

	withMissingMicrovm(&fakeAPIClient)
	_, err = reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when deleting microvm should not return error")

	_, err = getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestMicrovm_ReconcileDelete_GetReturnsNil(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()
	mvm.DeletionTimestamp = &metav1.Time{
		Time: time.Now(),
	}
	mvm.Finalizers = []string{infrav1.MvmFinalizer}

	fakeAPIClient := fakes.FakeClient{}
	withMissingMicrovm(&fakeAPIClient)

	client := createFakeClient(g, asRuntimeObject(mvm))

	result, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when deleting microvm should not return error")
	g.Expect(result.Requeue).To(BeFalse())
	g.Expect(result.RequeueAfter).To(Equal(time.Duration(0)))

	g.Expect(fakeAPIClient.DeleteMicroVMCallCount()).To(Equal(0))

	_, err = getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
}

func TestMicrovm_ReconcileDelete_GetErrors(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()
	mvm.DeletionTimestamp = &metav1.Time{
		Time: time.Now(),
	}
	mvm.Finalizers = []string{infrav1.MvmFinalizer}

	fakeAPIClient := fakes.FakeClient{}
	withExistingMicrovm(&fakeAPIClient, flintlocktypes.MicroVMStatus_CREATED)
	fakeAPIClient.GetMicroVMReturns(nil, errors.New("something terrible happened"))

	client := createFakeClient(g, asRuntimeObject(mvm))
	_, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).To(HaveOccurred(), "Reconciling when microvm service exists errors should return error")
}

func TestMicrovm_ReconcileDelete_DeleteErrors(t *testing.T) {
	g := NewWithT(t)

	mvm := createMicrovm()
	mvm.DeletionTimestamp = &metav1.Time{
		Time: time.Now(),
	}
	mvm.Finalizers = []string{infrav1.MvmFinalizer}

	fakeAPIClient := fakes.FakeClient{}
	withExistingMicrovm(&fakeAPIClient, flintlocktypes.MicroVMStatus_CREATED)
	fakeAPIClient.DeleteMicroVMReturns(nil, errors.New("something terrible happened"))

	client := createFakeClient(g, asRuntimeObject(mvm))
	_, err := reconcileMicrovm(client, &fakeAPIClient)
	g.Expect(err).To(HaveOccurred(), "Reconciling when deleting microvm errors should return error")

	reconciled, err := getMicrovm(client, testMicrovmName, testNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Getting microvm should not fail")

	assertConditionFalse(g, reconciled, infrav1.MicrovmReadyCondition, infrav1.MicrovmDeleteFailedReason)
	assertMicrovmNotReady(g, reconciled)
}
