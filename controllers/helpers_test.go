// Copyright 2022 Liquid Metal Authors or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MPL-2.0.

package controllers_test

import (
	"context"
	"encoding/base64"
	"fmt"

	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	flclient "github.com/liquidmetal-dev/controller-pkg/client"
	"github.com/liquidmetal-dev/controller-pkg/types/microvm"
	flintlockv1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	flintlocktypes "github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit/userdata"
	infrav1 "github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	"github.com/liquidmetal-dev/microvm-operator/controllers"
	"github.com/liquidmetal-dev/microvm-operator/controllers/fakes"
)

const (
	testNamespace             = "ns1"
	testMicrovmName           = "mvm1"
	testMicrovmReplicaSetName = "rs1"
	testMicrovmDeploymentName = "d1"
	testMicrovmUID            = "ABCDEF123456"
	testBootstrapData         = "somesamplebootstrapsdata"
)

func asRuntimeObject(microvm *infrav1.Microvm) []runtime.Object {
	objects := []runtime.Object{}

	if microvm != nil {
		objects = append(objects, microvm)
	}

	return objects
}

func reconcileMicrovm(client client.Client, mockAPIClient flclient.Client) (ctrl.Result, error) {
	mvmController := &controllers.MicrovmReconciler{
		Client: client,
		MvmClientFunc: func(address string, opts ...flclient.Options) (flclient.Client, error) {
			return mockAPIClient, nil
		},
	}

	request := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      testMicrovmName,
			Namespace: testNamespace,
		},
	}

	return mvmController.Reconcile(context.TODO(), request)
}

func reconcileMicrovmReplicaSet(client client.Client) (ctrl.Result, error) {
	mvmRSController := &controllers.MicrovmReplicaSetReconciler{
		Client: client,
		Scheme: client.Scheme(),
	}

	request := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      testMicrovmReplicaSetName,
			Namespace: testNamespace,
		},
	}

	return mvmRSController.Reconcile(context.TODO(), request)
}

func reconcileMicrovmReplicaSetNTimes(g *WithT, client client.Client, count int32) error {
	for count > 0 {
		ensureMicrovmState(g, client)
		if _, err := reconcileMicrovmReplicaSet(client); err != nil {
			return err
		}
		count--
	}

	return nil
}

func reconcileMicrovmDeployment(client client.Client) (ctrl.Result, error) {
	mvmDepController := &controllers.MicrovmDeploymentReconciler{
		Client: client,
		Scheme: client.Scheme(),
	}

	request := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      testMicrovmDeploymentName,
			Namespace: testNamespace,
		},
	}

	return mvmDepController.Reconcile(context.TODO(), request)
}

func reconcileMicrovmDeploymentNTimes(g *WithT, client client.Client, count int, r, rr int32) error {
	for count > 0 {
		ensureMicrovmReplicaSetState(g, client, r, rr)
		if _, err := reconcileMicrovmDeployment(client); err != nil {
			return err
		}
		count--
	}

	return nil
}

func getMicrovm(c client.Client, name, namespace string) (*infrav1.Microvm, error) {
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	mvm := &infrav1.Microvm{}
	err := c.Get(context.TODO(), key, mvm)
	return mvm, err
}

func listMicrovm(c client.Client) (*infrav1.MicrovmList, error) {
	mvm := &infrav1.MicrovmList{}
	err := c.List(context.TODO(), mvm)
	return mvm, err
}

func listMicrovmReplicaSet(c client.Client) (*infrav1.MicrovmReplicaSetList, error) {
	mvmRS := &infrav1.MicrovmReplicaSetList{}
	err := c.List(context.TODO(), mvmRS)
	return mvmRS, err
}

func getMicrovmReplicaSet(c client.Client, name, namespace string) (*infrav1.MicrovmReplicaSet, error) {
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	mvmRS := &infrav1.MicrovmReplicaSet{}
	err := c.Get(context.TODO(), key, mvmRS)
	return mvmRS, err
}

func getMicrovmDeployment(c client.Client, name, namespace string) (*infrav1.MicrovmDeployment, error) {
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	mvmD := &infrav1.MicrovmDeployment{}
	err := c.Get(context.TODO(), key, mvmD)
	return mvmD, err
}

func createFakeClient(g *WithT, objects []runtime.Object) client.Client {
	scheme := runtime.NewScheme()

	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())

	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
}

func createMicrovm() *infrav1.Microvm {
	return &infrav1.Microvm{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testMicrovmName,
			Namespace: testNamespace,
		},
		Spec: infrav1.MicrovmSpec{
			Host: microvm.Host{
				Endpoint: "127.0.0.1:9090",
			},
			ProviderID: pointer.String(testMicrovmUID),
			VMSpec: microvm.VMSpec{
				VCPU:     2,
				MemoryMb: 2048,
				RootVolume: microvm.Volume{
					Image:    "docker.io/richardcase/ubuntu-bionic-test:cloudimage_v0.0.1",
					ReadOnly: false,
				},
				Kernel: microvm.ContainerFileSource{
					Image:    "docker.io/richardcase/ubuntu-bionic-kernel:0.0.11",
					Filename: "vmlinuz",
				},
				Initrd: &microvm.ContainerFileSource{
					Image:    "docker.io/richardcase/ubuntu-bionic-kernel:0.0.11",
					Filename: "initrd-generic",
				},
				NetworkInterfaces: []microvm.NetworkInterface{
					{
						GuestDeviceName: "eth0",
						GuestMAC:        "",
						Type:            microvm.IfaceTypeMacvtap,
						Address:         "",
					},
				},
			},
		},
	}
}

func createMicrovmReplicaSet(reps int32) *infrav1.MicrovmReplicaSet {
	mvm := createMicrovm()
	mvm.Spec.Host = microvm.Host{}

	return &infrav1.MicrovmReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testMicrovmReplicaSetName,
			Namespace: testNamespace,
		},
		Spec: infrav1.MicrovmReplicaSetSpec{
			Host: microvm.Host{
				Endpoint: "127.0.0.1:9090",
			},
			Replicas: pointer.Int32(reps),
			Template: infrav1.MicrovmTemplateSpec{
				Spec: mvm.Spec,
			},
		},
	}
}

func createMicrovmDeployment(reps int32, hostCount int) *infrav1.MicrovmDeployment {
	mvm := createMicrovm()
	mvm.Spec.Host = microvm.Host{}

	var hosts []microvm.Host

	for i := 0; i < hostCount; i++ {
		hosts = append(hosts, microvm.Host{
			Endpoint: fmt.Sprintf("1.2.3.4:909%d", i),
		})
	}

	return &infrav1.MicrovmDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testMicrovmDeploymentName,
			Namespace: testNamespace,
		},
		Spec: infrav1.MicrovmDeploymentSpec{
			Hosts:    hosts,
			Replicas: pointer.Int32(reps),
			Template: infrav1.MicrovmTemplateSpec{
				Spec: mvm.Spec,
			},
		},
	}
}

func withExistingMicrovm(fc *fakes.FakeClient, mvmState flintlocktypes.MicroVMStatus_MicroVMState) {
	fc.GetMicroVMReturns(&flintlockv1.GetMicroVMResponse{
		Microvm: &flintlocktypes.MicroVM{
			Spec: &flintlocktypes.MicroVMSpec{
				Uid: pointer.String(testMicrovmUID),
			},
			Status: &flintlocktypes.MicroVMStatus{
				State: mvmState,
			},
		},
	}, nil)
}

func withMissingMicrovm(fc *fakes.FakeClient) {
	fc.GetMicroVMReturns(&flintlockv1.GetMicroVMResponse{}, nil)
}

func withCreateMicrovmSuccess(fc *fakes.FakeClient) {
	fc.CreateMicroVMReturns(&flintlockv1.CreateMicroVMResponse{
		Microvm: &flintlocktypes.MicroVM{
			Spec: &flintlocktypes.MicroVMSpec{
				Uid: pointer.String(testMicrovmUID),
			},
			Status: &flintlocktypes.MicroVMStatus{
				State: flintlocktypes.MicroVMStatus_PENDING,
			},
		},
	}, nil)
}

func assertConditionTrue(g *WithT, from conditions.Getter, conditionType clusterv1.ConditionType) {
	c := conditions.Get(from, conditionType)
	g.Expect(c).ToNot(BeNil(), "Conditions expected to be set")
	g.Expect(c.Status).To(Equal(corev1.ConditionTrue), "Condition should be marked true")
}

func assertConditionFalse(g *WithT, from conditions.Getter, conditionType clusterv1.ConditionType, reason string) {
	c := conditions.Get(from, conditionType)
	g.Expect(c).ToNot(BeNil(), "Conditions expected to be set")
	g.Expect(c.Status).To(Equal(corev1.ConditionFalse), "Condition should be marked false")
	g.Expect(c.Reason).To(Equal(reason))
}

func assertVMState(g *WithT, mvm *infrav1.Microvm, expectedState microvm.VMState) {
	g.Expect(mvm.Status.VMState).NotTo(BeNil())
	g.Expect(*mvm.Status.VMState).To(BeEquivalentTo(expectedState))
}

func assertMicrovmReconciled(g *WithT, reconciled *infrav1.Microvm) {
	assertConditionTrue(g, reconciled, infrav1.MicrovmReadyCondition)
	assertVMState(g, reconciled, microvm.VMStateRunning)
	assertFinalizer(g, reconciled)
	g.Expect(reconciled.Spec.ProviderID).ToNot(BeNil())
	expectedProviderID := fmt.Sprintf("microvm://127.0.0.1:9090/%s", testMicrovmUID)
	g.Expect(*reconciled.Spec.ProviderID).To(Equal(expectedProviderID))
	g.Expect(reconciled.Status.Ready).To(BeTrue(), "The Ready property must be true when the mvm has been reconciled")
}

func assertOneSetPerHost(g *WithT, reconciled *infrav1.MicrovmDeployment, c client.Client) {
	hosts := reconciled.Spec.Hosts
	sets, err := listMicrovmReplicaSet(c)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(len(hosts)).To(Equal(len(sets.Items)))

	seen := map[string]struct{}{}

	for _, rs := range sets.Items {
		seen[rs.Spec.Host.Endpoint] = struct{}{}
	}

	g.Expect(seen).To(HaveLen(len(hosts)))
}

func microvmsCreated(g *WithT, c client.Client) int32 {
	mvmList, err := listMicrovm(c)
	g.Expect(err).NotTo(HaveOccurred())
	return int32(len(mvmList.Items))
}

func microvmReplicaSetsCreated(g *WithT, c client.Client) int {
	mvmList, err := listMicrovmReplicaSet(c)
	g.Expect(err).NotTo(HaveOccurred())
	return len(mvmList.Items)
}

func ensureMicrovmState(g *WithT, c client.Client) {
	// update the microvms so they report as ready to move the replicaset reconciliation along
	mvmList, err := listMicrovm(c)
	g.Expect(err).NotTo(HaveOccurred())

	for _, mvm := range mvmList.Items {
		mvm.Status.Ready = true
		g.Expect(c.Update(context.TODO(), &mvm)).To(Succeed())
	}
}

func ensureMicrovmReplicaSetState(g *WithT, c client.Client, r, rr int32) {
	// update the microvmreplicasets so they report as ready to move the deployment reconciliation along
	mvmList, err := listMicrovmReplicaSet(c)
	g.Expect(err).NotTo(HaveOccurred())

	for _, mvm := range mvmList.Items {
		mvm.Status.Ready = true
		mvm.Status.ReadyReplicas = rr
		mvm.Status.Replicas = r
		g.Expect(c.Update(context.TODO(), &mvm)).To(Succeed())
	}
}

func assertFinalizer(g *WithT, reconciled *infrav1.Microvm) {
	g.Expect(reconciled.ObjectMeta.Finalizers).NotTo(BeEmpty(), "Expected at least one finalizer to be set")
	g.Expect(hasMicrovmFinalizer(&reconciled.ObjectMeta, infrav1.MvmFinalizer)).To(BeTrue(), "Expect the mvm finalizer")
}

func assertMRSFinalizer(g *WithT, reconciled *infrav1.MicrovmReplicaSet) {
	g.Expect(reconciled.ObjectMeta.Finalizers).NotTo(BeEmpty(), "Expected at least one finalizer to be set")
	g.Expect(hasMicrovmFinalizer(&reconciled.ObjectMeta, infrav1.MvmRSFinalizer)).To(BeTrue(), "Expect the mvmrs finalizer")
}

func assertMDFinalizer(g *WithT, reconciled *infrav1.MicrovmDeployment) {
	g.Expect(reconciled.ObjectMeta.Finalizers).NotTo(BeEmpty(), "Expected at least one finalizer to be set")
	g.Expect(hasMicrovmFinalizer(&reconciled.ObjectMeta, infrav1.MvmDeploymentFinalizer)).To(BeTrue(), "Expect the mvmd finalizer")
}

func hasMicrovmFinalizer(meta *metav1.ObjectMeta, finalizer string) bool {
	if len(meta.Finalizers) == 0 {
		return false
	}

	for _, f := range meta.Finalizers {
		if f == finalizer {
			return true
		}
	}

	return false
}

func assertMicrovmNotReady(g *WithT, mvm *infrav1.Microvm) {
	g.Expect(mvm.Status.Ready).To(BeFalse())
}

func assertVendorData(g *WithT, vendorDataRaw string, expectedSSHKeys []microvm.SSHPublicKey) {
	g.Expect(vendorDataRaw).ToNot(Equal(""))
	g.Expect(expectedSSHKeys).ToNot(BeNil())

	data, err := base64.StdEncoding.DecodeString(vendorDataRaw)
	g.Expect(err).NotTo(HaveOccurred(), "expect vendor data to be base64 encoded")

	vendorData := &userdata.UserData{}
	g.Expect(yaml.Unmarshal(data, vendorData)).To(Succeed(), "expect vendor data to unmarshall to cloud-init userdata")

	users := vendorData.Users
	g.Expect(users).NotTo(BeNil())
	g.Expect(len(users)).To(Equal(len(expectedSSHKeys)))

	for i, user := range users {
		g.Expect(user.Name).To(Equal(expectedSSHKeys[i].User))

		keys := user.SSHAuthorizedKeys
		g.Expect(keys).To(HaveLen(1))
		g.Expect(keys[0]).To(Equal(expectedSSHKeys[i].AuthorizedKeys[0]))
	}

	vendorDataStr := string(data)
	g.Expect(vendorDataStr).To(ContainSubstring("#cloud-config\n"))
}
