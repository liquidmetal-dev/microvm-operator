// Copyright 2022 Liquid Metal Authors or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MPL-2.0

package scope

import (
	"context"
	"fmt"

	flclient "github.com/liquidmetal-dev/controller-pkg/client"
	microvm "github.com/liquidmetal-dev/controller-pkg/types/microvm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	infrav1 "github.com/liquidmetal-dev/microvm-operator/api/v1alpha1"
	"github.com/liquidmetal-dev/microvm-operator/internal/defaults"
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
		return nil, fmt.Errorf("creating patch helper for microvm: %w", err)
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

// GetMicrovmSpec returns the spec for the MicroVM
func (m *MicrovmScope) GetMicrovmSpec() microvm.VMSpec {
	return m.MicroVM.Spec.VMSpec
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
func (m *MicrovmScope) GetSSHPublicKeys() []microvm.SSHPublicKey {
	if len(m.MicroVM.Spec.SSHPublicKeys) != 0 {
		return m.MicroVM.Spec.SSHPublicKeys
	}

	return nil
}

// GetLabels returns any user defined or default labels for the microvm.
func (m *MicrovmScope) GetLabels() map[string]string {
	return m.MicroVM.Spec.Labels
}

// GetRawBootstrapData will return any scripts intended to run on the microvm
func (m *MicrovmScope) GetRawBootstrapData() (string, error) {
	if m.MicroVM.Spec.UserData != nil {
		return *m.MicroVM.Spec.UserData, nil
	}

	return "#!/bin/bash\necho additional user data not supplied", nil
}

// GetBasicAuthToken will fetch the BasicAuthSecret from the cluster
// and return the token for the given host.
// If no secret or no value is found, an empty string is returned.
func (m *MicrovmScope) GetBasicAuthToken() (string, error) {
	if m.MicroVM.Spec.BasicAuthSecret == "" {
		return "", nil
	}

	tokenSecret := &corev1.Secret{}
	key := types.NamespacedName{
		Name:      m.MicroVM.Spec.BasicAuthSecret,
		Namespace: m.MicroVM.Namespace,
	}

	if err := m.client.Get(m.ctx, key, tokenSecret); err != nil {
		return "", err
	}

	// If it's not there, that's fine; we will log and return an empty string
	token := string(tokenSecret.Data["token"])

	if token == "" {
		m.Info(
			"basicAuthToken for host not found in secret", "secret", tokenSecret.Name,
		)
	}

	return token, nil
}

// GetTLSConfig will fetch the TLSSecretRef and CASecretRef for the MicroVM
// and return the TLS config for the client.
// If either are not set, it will be assumed that the host is not
// configured will TLS and all client calls will be made without credentials.
func (m *MicrovmScope) GetTLSConfig() (*flclient.TLSConfig, error) {
	if m.MicroVM.Spec.TLSSecretRef == "" {
		m.V(2).Info("no TLS configuration found. will create insecure connection")

		return nil, nil
	}

	secretKey := types.NamespacedName{
		Name:      m.MicroVM.Spec.TLSSecretRef,
		Namespace: m.MicroVM.Namespace,
	}

	tlsSecret := &corev1.Secret{}
	if err := m.client.Get(m.ctx, secretKey, tlsSecret); err != nil {
		return nil, err
	}

	certBytes, ok := tlsSecret.Data[tlsCert]
	if !ok {
		return nil, &tlsError{tlsCert}
	}

	keyBytes, ok := tlsSecret.Data[tlsKey]
	if !ok {
		return nil, &tlsError{tlsKey}
	}

	caBytes, ok := tlsSecret.Data[caCert]
	if !ok {
		return nil, &tlsError{caCert}
	}

	return &flclient.TLSConfig{
		Cert:   certBytes,
		Key:    keyBytes,
		CACert: caBytes,
	}, nil
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
