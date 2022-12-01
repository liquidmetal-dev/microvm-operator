// Copyright 2021 Weaveworks or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MPL-2.0

package v1alpha1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// MicrovmReadyCondition indicates that the microvm is in a running state.
	MicrovmReadyCondition clusterv1.ConditionType = "MicrovmReady"

	// MicrovmProvisionFailedReason indicates that the microvm failed to provision.
	MicrovmProvisionFailedReason = "MicrovmProvisionFailed"

	// MicrovmPendingReason indicates the microvm is in a pending state.
	MicrovmPendingReason = "MicrovmPending"

	// MicrovmDeletingReason indicates the microvm is in a deleted state.
	MicrovmDeletingReason = "MicrovmDeleting"

	// MicrovmDeletedFailedReason indicates the microvm failed to deleted cleanly.
	MicrovmDeleteFailedReason = "MicrovmDeleteFailed"

	// MicrovmUnknownStateReason indicates that the microvm in in an unknown or unsupported state
	// for reconciliation.
	MicrovmUnknownStateReason = "MicrovmUnknownState"

	// MicrovmReplicaSetReadyCondition indicates that the microvmreplicaset is in a complete state.
	MicrovmReplicaSetReadyCondition clusterv1.ConditionType = "MicrovmReplicaSetReady"

	// MicrovmReplicaSetIncompleteReason indicates the microvmreplicaset does not have all replicas yet.
	MicrovmReplicaSetIncompleteReason = "MicrovmReplicaSetIncomplete"

	// MicrovmReplicaSetProvisionFailedReason indicates that the microvm failed to provision.
	MicrovmReplicaSetProvisionFailedReason = "MicrovmReplicaSetProvisionFailed"

	// MicrovmReplicaSetDeletingReason indicates the microvmreplicaset is in a deleted state.
	MicrovmReplicaSetDeletingReason = "MicrovmReplicaSetDeleting"

	// MicrovmReplicaSetDeletedFailedReason indicates the microvmreplicaset failed to deleted cleanly.
	MicrovmReplicaSetDeleteFailedReason = "MicrovmReplicaSetDeleteFailed"

	// MicrovmReplicaSetUpdatingReason indicates the microvm is in a pending state.
	MicrovmReplicaSetUpdatingReason = "MicrovmReplicaSetUpdating"
)
