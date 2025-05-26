package v1alpha1

const (
	// Degraded is the condition type used to inform state of the operator when
	// it has failed with irrecoverable error like permission issues.
	// DebugEnabled has the following options:
	//   Status:
	//   - True
	//   - False
	//   Reason:
	//   - Failed
	Degraded string = "Degraded"

	// Ready is the condition type used to inform state of readiness of the
	// operator to process external-secrets enabling requests.
	//   Status:
	//   - True
	//   - False
	//   Reason:
	//   - Progressing
	//   - Failed
	//   - Ready: operand successfully deployed and ready
	Ready string = "Ready"
)

const (
	ReasonFailed string = "Failed"

	ReasonReady string = "Ready"

	ReasonInProgress string = "Progressing"
)
