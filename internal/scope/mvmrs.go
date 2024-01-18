// Copyright 2022 Liquid Metal Authors or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MPL-2.0

package scope

import (
	"context"
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

type MicrovmReplicaSetScopeParams struct {
	Logger            logr.Logger
	MicrovmReplicaSet *infrav1.MicrovmReplicaSet

	Client  client.Client
	Context context.Context //nolint: containedctx // don't care
}

type MicrovmReplicaSetScope struct {
	logr.Logger

	MicrovmReplicaSet *infrav1.MicrovmReplicaSet

	client         client.Client
	patchHelper    *patch.Helper
	controllerName string
	ctx            context.Context
}

func NewMicrovmReplicaSetScope(params MicrovmReplicaSetScopeParams) (*MicrovmReplicaSetScope, error) {
	if params.MicrovmReplicaSet == nil {
		return nil, errMicrovmRequired
	}

	if params.Client == nil {
		return nil, errClientRequired
	}

	patchHelper, err := patch.NewHelper(params.MicrovmReplicaSet, params.Client)
	if err != nil {
		return nil, fmt.Errorf("creating patch helper for microvmreplicaset: %w", err)
	}

	scope := &MicrovmReplicaSetScope{
		MicrovmReplicaSet: params.MicrovmReplicaSet,
		client:            params.Client,
		controllerName:    defaults.ManagerName,
		Logger:            params.Logger,
		patchHelper:       patchHelper,
		ctx:               params.Context,
	}

	return scope, nil
}

// Name returns the MicrovmReplicaSet name.
func (m *MicrovmReplicaSetScope) Name() string {
	return m.MicrovmReplicaSet.Name
}

// Namespace returns the namespace name.
func (m *MicrovmReplicaSetScope) Namespace() string {
	return m.MicrovmReplicaSet.Namespace
}

// DesiredReplicas returns requested replicas set on the spec.
func (m *MicrovmReplicaSetScope) DesiredReplicas() int32 {
	return *m.MicrovmReplicaSet.Spec.Replicas
}

// ReadyReplicas returns the number of replicas which are ready.
func (m *MicrovmReplicaSetScope) ReadyReplicas() int32 {
	return *&m.MicrovmReplicaSet.Status.ReadyReplicas
}

// CreatedReplicas returns the number of replicas which have been created.
func (m *MicrovmReplicaSetScope) CreatedReplicas() int32 {
	return *&m.MicrovmReplicaSet.Status.Replicas
}

// GetMicrovmSpec returns the spec for the child MicroVM
func (m *MicrovmReplicaSetScope) MicrovmSpec() infrav1.MicrovmSpec {
	return m.MicrovmReplicaSet.Spec.Template.Spec
}

// GetMicrovmHost returns the host for the child MicroVM
func (m *MicrovmReplicaSetScope) MicrovmHost() microvm.Host {
	return m.MicrovmReplicaSet.Spec.Host
}

// SetCreatedReplicas records the number of microvms which have been created
// this does not give information about whether the microvms are ready
func (m *MicrovmReplicaSetScope) SetCreatedReplicas(count int32) {
	m.MicrovmReplicaSet.Status.Replicas = count
}

// SetReadyReplicas saves the number of ready MicroVMs to the status
func (m *MicrovmReplicaSetScope) SetReadyReplicas(count int32) {
	m.MicrovmReplicaSet.Status.ReadyReplicas = count
}

// SetReady sets any properties/conditions that are used to indicate that the Microvm is 'Ready'.
func (m *MicrovmReplicaSetScope) SetReady() {
	conditions.MarkTrue(m.MicrovmReplicaSet, infrav1.MicrovmReplicaSetReadyCondition)
	m.MicrovmReplicaSet.Status.Ready = true
}

// SetNotReady sets any properties/conditions that are used to indicate that the MicrovmReplicaSet is NOT 'Ready'.
func (m *MicrovmReplicaSetScope) SetNotReady(
	reason string,
	severity clusterv1.ConditionSeverity,
	message string,
	messageArgs ...interface{},
) {
	conditions.MarkFalse(m.MicrovmReplicaSet, infrav1.MicrovmReplicaSetReadyCondition, reason, severity, message, messageArgs...)
	m.MicrovmReplicaSet.Status.Ready = false
}

// Patch persists the resource and status.
func (m *MicrovmReplicaSetScope) Patch() error {
	err := m.patchHelper.Patch(
		m.ctx,
		m.MicrovmReplicaSet,
	)
	if err != nil {
		return fmt.Errorf("unable to patch microvmreplicaset: %w", err)
	}

	return nil
}
