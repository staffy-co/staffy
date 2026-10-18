package domain_test

import (
	"reflect"
	"testing"

	"github.com/artemiosu/staffy/internal/action/domain"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	controldomain "github.com/artemiosu/staffy/internal/control/domain"
)

// The mirror contract of the control domain: the dictionaries that mirror
// the living sources are pinned here — drift turns one of these tests red
// (the control domain never imports the source packages; this test
// imports BOTH sides and compares). Pinning levels are honest and
// explicit:
//   - full-list equality in the canonical order (additions, renames and
//     removals caught): channels, policy outcome classes, autonomy
//     classes;
//   - the AD-12 mode words are pinned by model_test at unit level AND by
//     the OpenAPI contract test of step-03 — the exact string with the
//     slash is the load-bearing pin (renaming intake_only/no_promises to
//     intake_only is a contract violation, never a cleanup).

// TestChannelWordsMirrorTheCasesChannelOrder pins the nine profile
// channels against the canonical order of the cases dictionary.
func TestChannelWordsMirrorTheCasesChannelOrder(t *testing.T) {
	if got, want := controldomain.ChannelWords, casesDomain.ChannelOrder; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало каналов разошлось со словарём cases:\nконтроль: %v\nисточник: %v", got, want)
	}
}

// TestPolicyOutcomeWordsMirrorTheCasesOutcomeClasses pins the outcome
// classes of 2-6 — the living dictionary carries six words (degraded_mode
// joined in 2-12); the mirror follows the source.
func TestPolicyOutcomeWordsMirrorTheCasesOutcomeClasses(t *testing.T) {
	if got, want := controldomain.PolicyOutcomeWords, casesDomain.OutcomeClasses; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало классов исхода разошлось с cases 2-6:\nконтроль: %v\nисточник: %v", got, want)
	}
}

// TestAutonomyClassWordsMirrorTheActionDictionary pins the FR-42 five
// words against the action domain of 3-1 — the same vocabulary the
// action catalog guards.
func TestAutonomyClassWordsMirrorTheActionDictionary(t *testing.T) {
	if got, want := controldomain.AutonomyClassWords, domain.AutonomyClasses; !reflect.DeepEqual(got, want) {
		t.Fatalf("зеркало классов автономии разошлось с action 3-1:\nконтроль: %v\nисточник: %v", got, want)
	}
}
