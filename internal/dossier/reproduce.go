package dossier

// Reproducibility posture: whether a dossier carries every input the
// server-side reproduce routine needs to deterministically re-run the
// decision. A shape/coverage check only — it never claims the verdict did
// reproduce. Faithful port of @decionis/verify's assessDossierReproducibility.

// ReproducibilityInput mirrors the upstream shape.
type ReproducibilityInput struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Present bool    `json:"present"`
	Value   *string `json:"value"`
}

// ReproducibilityAssessment mirrors the upstream shape. Posture is one of
// reproduction_ready | incomplete | signature_only.
type ReproducibilityAssessment struct {
	Posture string                 `json:"posture"`
	Detail  string                 `json:"detail"`
	Inputs  []ReproducibilityInput `json:"inputs"`
}

func optString(record map[string]any, key string) *string {
	if s, ok := asString(record[key]); ok {
		return &s
	}
	return nil
}

func firstString(candidates ...*string) *string {
	for _, candidate := range candidates {
		if candidate != nil {
			return candidate
		}
	}
	return nil
}

// AssessReproducibility reports whether the dossier payload carries the
// recorded outcome, policy version, rules hash, evaluation timestamp, and
// inputs snapshot the reproduce routine reads.
func AssessReproducibility(payload map[string]any) ReproducibilityAssessment {
	policySnapshot := asRecord(asRecord(payload["governance"])["policy_snapshot"])
	routing := asRecord(payload["routing_decision"])

	recordedOutcome := optString(routing, "outcome")
	policyVersion := firstString(optString(policySnapshot, "policy_version"), optString(routing, "policy_version"))
	rulesSha := optString(policySnapshot, "rules_sha256")
	evaluatedAt := firstString(
		optString(policySnapshot, "evaluated_at"),
		optString(asRecord(routing["policy_evaluation"]), "evaluated_at"),
		optString(payload, "generated_at"),
	)
	inputsSnapshot, hasInputs := payload["inputs_snapshot"].(map[string]any)
	inputsPresent := hasInputs && len(inputsSnapshot) > 0

	inputs := []ReproducibilityInput{
		{Key: "recorded_outcome", Label: "Recorded outcome", Present: recordedOutcome != nil, Value: recordedOutcome},
		{Key: "policy_version", Label: "Policy version", Present: policyVersion != nil, Value: policyVersion},
		{Key: "rules_sha256", Label: "Recorded rules hash", Present: rulesSha != nil, Value: rulesSha},
		{Key: "evaluated_at", Label: "Evaluation timestamp", Present: evaluatedAt != nil, Value: evaluatedAt},
		{Key: "inputs_snapshot", Label: "Inputs snapshot", Present: inputsPresent, Value: nil},
	}

	presentCount := 0
	var missing []string
	for _, input := range inputs {
		if input.Present {
			presentCount++
		} else {
			missing = append(missing, input.Label)
		}
	}

	switch presentCount {
	case len(inputs):
		return ReproducibilityAssessment{
			Posture: "reproduction_ready",
			Detail: "Carries the recorded policy version, rules hash, evaluation time, outcome, and inputs — " +
				"the engine can re-run this decision and hash-match the exact recorded bundle.",
			Inputs: inputs,
		}
	case 0:
		return ReproducibilityAssessment{
			Posture: "signature_only",
			Detail: "No reproduction metadata present. The signature can still be verified, but the verdict " +
				"cannot be independently recomputed — this is a signed record, not a reproducible one.",
			Inputs: inputs,
		}
	default:
		detail := "Missing reproduction inputs: "
		for i, label := range missing {
			if i > 0 {
				detail += ", "
			}
			detail += label
		}
		detail += ". Reproduction would return indeterminate."
		return ReproducibilityAssessment{Posture: "incomplete", Detail: detail, Inputs: inputs}
	}
}
