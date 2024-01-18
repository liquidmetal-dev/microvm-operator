// Copyright 2021 Liquid Metal Authors or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MPL-2.0

package controllers

import "errors"

var (
	// errControlplaneEndpointRequired = errors.New("controlplane endpoint is required on cluster or mvmcluster")
	errClientFactoryFuncRequired = errors.New("factory function required to create grpc client")
	errMicrovmFailed             = errors.New("microvm is in a failed state")
	errMicrovmUnknownState       = errors.New("microvm is in an unknown/unsupported state")
	// errNoPlacement                  = errors.New("no placement specified")
)
