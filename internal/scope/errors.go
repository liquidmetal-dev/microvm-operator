// Copyright 2022 Liquid Metal Authors or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MPL-2.0

package scope

import "errors"

var (
	errMicrovmRequired = errors.New("microvm required to create scope")
	errClientRequired  = errors.New("controller-runtime client required to create scope")
)

type tlsError struct {
	key string
}

func (t *tlsError) Error() string {
	return "required key missing from TLS config data: " + t.key
}
