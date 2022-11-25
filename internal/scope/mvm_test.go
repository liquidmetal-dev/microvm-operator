package scope_test

import (
	"testing"

	"github.com/go-logr/logr/testr"
	. "github.com/onsi/gomega"

	flclient "github.com/weaveworks-liquidmetal/controller-pkg/client"
	"github.com/weaveworks-liquidmetal/controller-pkg/types/microvm"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "github.com/weaveworks-liquidmetal/microvm-operator/api/v1alpha1"
	"github.com/weaveworks-liquidmetal/microvm-operator/internal/scope"
)

func TestMicrovmProviderID(t *testing.T) {
	RegisterTestingT(t)

	scheme, err := setupScheme()
	Expect(err).NotTo(HaveOccurred())

	mvmName := "m-1"
	mvm := newMicrovm(mvmName, "")

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mvm).Build()
	mvmScope, err := scope.NewMicrovmScope(scope.MicrovmScopeParams{
		Client:  client,
		MicroVM: mvm,
	})
	Expect(err).NotTo(HaveOccurred())

	mvmScope.SetProviderID("abcdef")
	Expect(mvmScope.GetProviderID()).To(Equal("microvm://fd1/abcdef"))
}

func TestMicrovmGetInstanceID(t *testing.T) {
	RegisterTestingT(t)

	scheme, err := setupScheme()
	Expect(err).NotTo(HaveOccurred())

	mvmName := "m-1"
	uid := "abcdef"
	providerID := "microvm://fd1/" + uid
	mvm := newMicrovm(mvmName, providerID)

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mvm).Build()
	mvmScope, err := scope.NewMicrovmScope(scope.MicrovmScopeParams{
		Client:  client,
		MicroVM: mvm,
	})
	Expect(err).NotTo(HaveOccurred())

	instanceID := mvmScope.GetInstanceID()
	Expect(instanceID).To(Equal(uid))
}

// This is all temporary
func TestMicrovmGetBasicAuthToken(t *testing.T) {
	RegisterTestingT(t)

	scheme, err := setupScheme()
	Expect(err).NotTo(HaveOccurred())

	mvmName := "testcluster"
	secretName := "testsecret"
	hostName := "hostwiththemost"
	token := "foo"

	mvm := newMicrovmWithSpec(mvmName, infrav1.MicrovmSpec{
		Host: infrav1.HostSpec{
			Host: microvm.Host{
				Endpoint: hostName,
			},
			BasicAuthSecret: secretName,
		},
	})
	otherMvm := newMicrovm(mvmName, "")
	secret := newSecret(secretName, map[string][]byte{"token": []byte(token)})
	otherSecret := newSecret(secretName, map[string][]byte{"nottoken": []byte(token)})

	tt := []struct {
		name        string
		expected    string
		expectedErr func(error)
		initObjects []client.Object
		mvm         *infrav1.Microvm
	}{
		{
			name: "when the token is found in the secret, it is returned",
			initObjects: []client.Object{
				mvm, secret,
			},
			mvm:      mvm,
			expected: token,
			expectedErr: func(err error) {
				Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name:        "when the secret does not exist, returns the error",
			initObjects: []client.Object{mvm},
			mvm:         mvm,
			expected:    "",
			expectedErr: func(err error) {
				Expect(err).To(HaveOccurred())
			},
		},
		{
			name:        "when the secret does not contain the token, an empty string is returned",
			initObjects: []client.Object{mvm, otherSecret},
			mvm:         mvm,
			expected:    "",
			expectedErr: func(err error) {
				Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			name:        "when the secret name is not set on the cluster, empty string is returned",
			initObjects: []client.Object{otherMvm},
			mvm:         otherMvm,
			expected:    "",
			expectedErr: func(err error) {
				Expect(err).NotTo(HaveOccurred())
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.initObjects...).Build()
			mvmScope, err := scope.NewMicrovmScope(scope.MicrovmScopeParams{
				Client:  client,
				MicroVM: tc.mvm,
				Logger:  testr.New(t),
			})
			Expect(err).NotTo(HaveOccurred())

			token, err := mvmScope.GetBasicAuthToken()
			tc.expectedErr(err)
			Expect(token).To(Equal(tc.expected))
		})
	}
}

func TestMicrovmGetTLSConfig(t *testing.T) {
	RegisterTestingT(t)

	scheme, err := setupScheme()
	Expect(err).NotTo(HaveOccurred())

	mvmName := "testmvm"
	tlsSecretName := "testtlssecret"

	mvm := newMicrovmWithSpec(mvmName, infrav1.MicrovmSpec{
		TLSSecretRef: tlsSecretName,
	})
	otherMvmNoTLS := newMicrovm(mvmName, "")

	tlsData := map[string][]byte{
		"tls.crt": []byte("foo"),
		"tls.key": []byte("bar"),
		"ca.crt":  []byte("baz"),
	}
	tlsSecret := newSecret(tlsSecretName, tlsData)

	badData := map[string][]byte{
		"not": []byte("great"),
	}
	otherTLSSecret := newSecret(tlsSecretName, badData)

	tt := []struct {
		name        string
		expected    func(*flclient.TLSConfig, error)
		initObjects []client.Object
		mvm         *infrav1.Microvm
	}{
		{
			name: "returns the TLS config from the secret",
			initObjects: []client.Object{
				mvm, tlsSecret,
			},
			mvm: mvm,
			expected: func(cfg *flclient.TLSConfig, err error) {
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				Expect(cfg.Cert).To(Equal([]byte("foo")))
				Expect(cfg.Key).To(Equal([]byte("bar")))
				Expect(cfg.CACert).To(Equal([]byte("baz")))
			},
		},
		{
			name: "when the tls secret does not exist, returns an error",
			initObjects: []client.Object{
				mvm,
			},
			mvm: mvm,
			expected: func(cfg *flclient.TLSConfig, err error) {
				Expect(err).To(HaveOccurred())
			},
		},
		{
			name: "when the TLSSecretRef is not set on the microvm, returns nil",
			initObjects: []client.Object{
				otherMvmNoTLS,
			},
			mvm: otherMvmNoTLS,
			expected: func(cfg *flclient.TLSConfig, err error) {
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg).To(BeNil())
			},
		},
		{
			name: "when the secret data does not contain the `tls.crt` key, returns an error",
			initObjects: []client.Object{
				mvm, otherTLSSecret,
			},
			mvm: mvm,
			expected: func(cfg *flclient.TLSConfig, err error) {
				Expect(err).To(HaveOccurred())
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.initObjects...).Build()
			mvm, err := scope.NewMicrovmScope(scope.MicrovmScopeParams{
				Client:  client,
				MicroVM: tc.mvm,
				Logger:  testr.New(t),
			})
			Expect(err).NotTo(HaveOccurred())

			tc.expected(mvm.GetTLSConfig())
		})
	}
}

func setupScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := infrav1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := clusterv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

func newCluster(name string, failureDomains []string) *clusterv1.Cluster {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}

	if len(failureDomains) > 0 {
		cluster.Status = clusterv1.ClusterStatus{
			FailureDomains: make(clusterv1.FailureDomains),
		}

		for i := range failureDomains {
			fd := failureDomains[i]
			cluster.Status.FailureDomains[fd] = clusterv1.FailureDomainSpec{
				ControlPlane: true,
			}
		}
	}

	return cluster
}

func newMicrovm(name string, providerID string) *infrav1.Microvm {
	mvm := &infrav1.Microvm{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: infrav1.MicrovmSpec{
			Host: infrav1.HostSpec{
				Host: microvm.Host{
					Endpoint: "fd1",
				},
			},
		},
	}
	if providerID != "" {
		mvm.Spec.ProviderID = &providerID
	}

	return mvm
}

func newMicrovmWithSpec(name string, spec infrav1.MicrovmSpec) *infrav1.Microvm {
	mvm := newMicrovm(name, "")
	mvm.Spec = spec
	return mvm
}

func newSecret(name string, data map[string][]byte) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Data: data,
	}
}
