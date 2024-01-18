// Copyright 2022 Liquid Metal Authors or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MPL-2.0

package scope

import (
	"context"
	"errors"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	microvm "github.com/liquidmetal-dev/controller-pkg/types/microvm"
	infrav1 "github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	"github.com/liquidmetal-dev/microvm-operator/internal/defaults"
)

type MicrovmDeploymentScopeParams struct {
	Logger            logr.Logger
	MicrovmDeployment *infrav1.MicrovmDeployment

	Client  client.Client
	Context context.Context //nolint: containedctx // don't care
}

type MicrovmDeploymentScope struct {
	logr.Logger

	MicrovmDeployment *infrav1.MicrovmDeployment

	client         client.Client
	patchHelper    *patch.Helper
	controllerName string
	ctx            context.Context
}

func NewMicrovmDeploymentScope(params MicrovmDeploymentScopeParams) (*MicrovmDeploymentScope, error) {
	if params.MicrovmDeployment == nil {
		return nil, errMicrovmRequired
	}

	if params.Client == nil {
		return nil, errClientRequired
	}

	patchHelper, err := patch.NewHelper(params.MicrovmDeployment, params.Client)
	if err != nil {
		return nil, fmt.Errorf("creating patch helper for microvmreplicaset: %w", err)
	}

	scope := &MicrovmDeploymentScope{
		MicrovmDeployment: params.MicrovmDeployment,
		client:            params.Client,
		controllerName:    defaults.ManagerName,
		Logger:            params.Logger,
		patchHelper:       patchHelper,
		ctx:               params.Context,
	}

	return scope, nil
}

// Name returns the MicrovmDeployment name.
func (m *MicrovmDeploymentScope) Name() string {
	return m.MicrovmDeployment.Name
}

// Namespace returns the namespace name.
func (m *MicrovmDeploymentScope) Namespace() string {
	return m.MicrovmDeployment.Namespace
}

// HasAllSets returns true if all required sets have been created
func (m *MicrovmDeploymentScope) HasAllSets(count int) bool {
	return count == len(m.MicrovmDeployment.Spec.Hosts)
}

// RequiredSets returns the number of sets which should be created
func (m *MicrovmDeploymentScope) RequiredSets() int {
	return len(m.MicrovmDeployment.Spec.Hosts)
}

// DesiredTotalReplicas returns the toal requested replicas set on the spec.
func (m *MicrovmDeploymentScope) DesiredTotalReplicas() int32 {
	return m.DesiredReplicas() * int32(m.RequiredSets())
}

// DesiredReplicas returns the requested replicas set per set on the spec.
func (m *MicrovmDeploymentScope) DesiredReplicas() int32 {
	return *m.MicrovmDeployment.Spec.Replicas
}

// ReadyReplicas returns the number of replicas which are ready.
func (m *MicrovmDeploymentScope) ReadyReplicas() int32 {
	return *&m.MicrovmDeployment.Status.ReadyReplicas
}

// CreatedReplicas returns the number of replicas which have been created.
func (m *MicrovmDeploymentScope) CreatedReplicas() int32 {
	return *&m.MicrovmDeployment.Status.Replicas
}

// GetMicrovmSpec returns the spec for the child MicroVM
func (m *MicrovmDeploymentScope) MicrovmSpec() infrav1.MicrovmSpec {
	return m.MicrovmDeployment.Spec.Template.Spec
}

// Hosts returns the list of hosts for created microvms
func (m *MicrovmDeploymentScope) Hosts() []microvm.Host {
	return m.MicrovmDeployment.Spec.Hosts
}

// DetermineHost returns a host which does not yet have a replicaset
func (m *MicrovmDeploymentScope) DetermineHost(setHosts infrav1.HostMap) (microvm.Host, error) {
	for _, host := range m.Hosts() {
		if _, ok := setHosts[host.Endpoint]; !ok {
			return host, nil
		}
	}

	return microvm.Host{}, errors.New("could not find free host")
}

// ExpiredHosts returns hosts which have been removed from the spec
func (m *MicrovmDeploymentScope) ExpiredHosts(setHosts infrav1.HostMap) infrav1.HostMap {
	for _, host := range m.Hosts() {
		delete(setHosts, host.Endpoint)
	}

	return setHosts
}

// SetCreatedReplicas records the number of microvms which have been created
// this does not give information about whether the microvms are ready
func (m *MicrovmDeploymentScope) SetCreatedReplicas(count int32) {
	m.MicrovmDeployment.Status.Replicas = count
}

// SetReadyReplicas saves the number of ready MicroVMs to the status
func (m *MicrovmDeploymentScope) SetReadyReplicas(count int32) {
	m.MicrovmDeployment.Status.ReadyReplicas = count
}

// SetReady sets any properties/conditions that are used to indicate that the Microvm is 'Ready'.
func (m *MicrovmDeploymentScope) SetReady() {
	conditions.MarkTrue(m.MicrovmDeployment, infrav1.MicrovmDeploymentReadyCondition)
	m.MicrovmDeployment.Status.Ready = true
}

// SetNotReady sets any properties/conditions that are used to indicate that the MicrovmDeployment is NOT 'Ready'.
func (m *MicrovmDeploymentScope) SetNotReady(
	reason string,
	severity clusterv1.ConditionSeverity,
	message string,
	messageArgs ...interface{},
) {
	conditions.MarkFalse(m.MicrovmDeployment, infrav1.MicrovmDeploymentReadyCondition, reason, severity, message, messageArgs...)
	m.MicrovmDeployment.Status.Ready = false
}

// Patch persists the resource and status.
func (m *MicrovmDeploymentScope) Patch() error {
	err := m.patchHelper.Patch(
		m.ctx,
		m.MicrovmDeployment,
	)
	if err != nil {
		return fmt.Errorf("unable to patch microvmreplicaset: %w", err)
	}

	return nil
}
