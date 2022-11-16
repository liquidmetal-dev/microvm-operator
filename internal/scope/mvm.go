// Copyright 2022 Weaveworks or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MPL-2.0

package scope

import (
	"context"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	infrav1 "github.com/weaveworks-liquidmetal/microvm-operator/api/v1alpha1"
	"github.com/weaveworks-liquidmetal/microvm-operator/internal/defaults"
)

const ProviderPrefix = "microvm://"

const (
	tlsCert = "tls.crt"
	tlsKey  = "tls.key"
	caCert  = "ca.crt"
)

type MicrovmScopeParams struct {
	Logger  logr.Logger
	MicroVM *infrav1.Microvm

	Client  client.Client
	Context context.Context //nolint: containedctx // don't care
}

type MicrovmScope struct {
	logr.Logger

	MicroVM *infrav1.Microvm

	client         client.Client
	patchHelper    *patch.Helper
	controllerName string
	ctx            context.Context
}

func NewMicrovmScope(params MicrovmScopeParams) (*MicrovmScope, error) {
	if params.MicroVM == nil {
		return nil, errMicrovmRequired
	}

	if params.Client == nil {
		return nil, errClientRequired
	}

	patchHelper, err := patch.NewHelper(params.MicroVM, params.Client)
	if err != nil {
		return nil, fmt.Errorf("creating patch helper for microvm machine: %w", err)
	}

	scope := &MicrovmScope{
		MicroVM:        params.MicroVM,
		client:         params.Client,
		controllerName: defaults.ManagerName,
		Logger:         params.Logger,
		patchHelper:    patchHelper,
		ctx:            params.Context,
	}

	return scope, nil
}

// Name returns the Microvm name.
func (m *MicrovmScope) Name() string {
	return m.MicroVM.Name
}

// Namespace returns the namespace name.
func (m *MicrovmScope) Namespace() string {
	return m.MicroVM.Namespace
}

// GetInstanceID gets the instance ID (i.e. UID) of the mvm.
func (m *MicrovmScope) GetInstanceID() string {
	parsed, err := noderefutil.NewProviderID(m.GetProviderID())
	if err != nil {
		return ""
	}

	return parsed.ID()
}

// SetProviderID saves the unique microvm and object ID to the Mvm spec.
func (m *MicrovmScope) SetProviderID(mvmUID string) {
	providerID := fmt.Sprintf("%s%s/%s", ProviderPrefix, m.MicroVM.Spec.Host.Endpoint, mvmUID)
	m.MicroVM.Spec.ProviderID = &providerID
}

// GetProviderID returns the provider if for the vm. If there is no provider id
// then an empty string will be returned.
func (m *MicrovmScope) GetProviderID() string {
	if m.MicroVM.Spec.ProviderID != nil {
		return *m.MicroVM.Spec.ProviderID
	}

	return ""
}

// GetSSHPublicKeys will return the SSH public keys for this vm.
func (m *MicrovmScope) GetSSHPublicKeys() []infrav1.SSHPublicKey {
	if len(m.MicroVM.Spec.SSHPublicKeys) != 0 {
		return m.MicroVM.Spec.SSHPublicKeys
	}

	return nil
}

// GetAdditionalUserData will return any scripts intended to run on the microvm
func (m *MicrovmScope) GetAdditionalUserData() string {
	if m.MicroVM.Spec.UserData != nil {
		return *m.MicroVM.Spec.UserData
	}

	return "#!/bin/bash\necho additional user data not supplied"
}

// SetReady sets any properties/conditions that are used to indicate that the Microvm is 'Ready'.
func (m *MicrovmScope) SetReady() {
	conditions.MarkTrue(m.MicroVM, infrav1.MicrovmReadyCondition)
	m.MicroVM.Status.Ready = true
}

// SetNotReady sets any properties/conditions that are used to indicate that the Microvm is NOT 'Ready'.
func (m *MicrovmScope) SetNotReady(
	reason string,
	severity clusterv1.ConditionSeverity,
	message string,
	messageArgs ...interface{},
) {
	conditions.MarkFalse(m.MicroVM, infrav1.MicrovmReadyCondition, reason, severity, message, messageArgs...)
	m.MicroVM.Status.Ready = false
}

// Patch persists the resource and status.
func (m *MicrovmScope) Patch() error {
	err := m.patchHelper.Patch(
		m.ctx,
		m.MicroVM,
	)
	if err != nil {
		return fmt.Errorf("unable to patch microvm: %w", err)
	}

	return nil
}
