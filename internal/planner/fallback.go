package planner

// The approved safe fallback forms of a failed turn (Story 8-1, AD-7
// verbatim: «A second validation failure is persisted and yields only
// an approved safe fallback, handoff or callback—never model output»).
// The three forms are closed platform words — honest, vertical-clean
// formulations with no domain vocabulary. The CONTEXT declares which
// form applies (the fallback policy of the turn — chosen by the future
// context builder of Story 8.4); the machine only executes the
// declared policy. The fallback material is NEVER the model output in
// any sense: the failed proposal is discarded, the journal keeps the
// named cause, and the turn ends with the platform form alone.

// FallbackForm is the approved safe form that ends a failed turn.
type FallbackForm struct {
	// Kind is the closed word: honest_stop | handoff | callback.
	Kind string
}

// ApprovedFallback resolves the declared fallback policy of the
// context into the approved safe form. The policy word and the form
// word are the same closed dictionary — the resolution makes the
// approval explicit and keeps a foreign word impossible.
func ApprovedFallback(policy string) (FallbackForm, error) {
	if !isDictionaryWord(policy, FallbackDictionary) {
		return FallbackForm{}, rule("context.fallback_policy", "fallback-политика %q вне закрытого словаря утверждённых безопасных форм", policy)
	}
	return FallbackForm{Kind: policy}, nil
}
