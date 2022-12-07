package scope_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	microvm "github.com/weaveworks-liquidmetal/controller-pkg/types/microvm"
	infrav1 "github.com/weaveworks-liquidmetal/microvm-operator/api/v1alpha1"
	"github.com/weaveworks-liquidmetal/microvm-operator/internal/scope"
)

func TestDetermineHost(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	mvmDepName := "md-1"

	tt := []struct {
		name      string
		expected  func(*WithT, string, string, error)
		hostCount int
		mapCount  int
	}{
		{
			name:      "when a host is not yet recorded in the map, should return that host",
			hostCount: 5,
			mapCount:  3,
			expected: func(g *WithT, wantHost, gotHost string, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gotHost).To(Equal(wantHost))
			},
		},
		{
			name:      "testing the same but with different numbers just in case",
			hostCount: 10,
			mapCount:  4,
			expected: func(g *WithT, wantHost, gotHost string, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(gotHost).To(Equal(wantHost))
			},
		},
		{
			name:      "when there is no unmapped host to return, return error",
			hostCount: 2,
			mapCount:  2,
			expected: func(g *WithT, _, _ string, err error) {
				g.Expect(err).To(MatchError("could not find free host"))
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			mvmDep := newDeployment(mvmDepName, tc.hostCount)

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mvmDep).Build()
			mvmScope, err := scope.NewMicrovmDeploymentScope(scope.MicrovmDeploymentScopeParams{
				Client:            client,
				MicrovmDeployment: mvmDep,
			})
			g.Expect(err).NotTo(HaveOccurred())

			hostMap := newHostMap(tc.mapCount)

			host, err := mvmScope.DetermineHost(hostMap)
			tc.expected(g, fmt.Sprintf("%d", tc.mapCount), host.Endpoint, err)
		})
	}
}

func TestExpiredHosts(t *testing.T) {
	g := NewWithT(t)

	scheme, err := setupScheme()
	g.Expect(err).NotTo(HaveOccurred())

	mvmDepName := "md-1"

	mvmDep := newDeployment(mvmDepName, 0)
	mvmDep.Spec.Hosts = []microvm.Host{
		{Endpoint: "1"}, {Endpoint: "2"},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mvmDep).Build()
	mvmScope, err := scope.NewMicrovmDeploymentScope(scope.MicrovmDeploymentScopeParams{
		Client:            client,
		MicrovmDeployment: mvmDep,
	})
	g.Expect(err).NotTo(HaveOccurred())

	hostMap := infrav1.HostMap{
		"1": struct{}{},
		"2": struct{}{},
		"3": struct{}{},
		"4": struct{}{},
	}

	hosts := mvmScope.ExpiredHosts(hostMap)
	g.Expect(len(hosts)).To(Equal(2))
	g.Expect(hostMap).ToNot(HaveKey("1"))
	g.Expect(hostMap).ToNot(HaveKey("2"))
	g.Expect(hostMap).To(HaveKey("3"))
	g.Expect(hostMap).To(HaveKey("4"))
}

func newHostMap(hostCount int) infrav1.HostMap {
	hostMap := infrav1.HostMap{}
	for i := 0; i < hostCount; i++ {
		hostMap[fmt.Sprintf("%d", i)] = struct{}{}
	}

	return hostMap
}

func newDeployment(name string, hostCount int) *infrav1.MicrovmDeployment {
	var hosts []microvm.Host

	for i := 0; i < hostCount; i++ {
		hosts = append(hosts, microvm.Host{
			Endpoint: fmt.Sprintf("%d", i),
		})
	}

	md := &infrav1.MicrovmDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: infrav1.MicrovmDeploymentSpec{
			Hosts: hosts,
		},
	}

	return md
}
