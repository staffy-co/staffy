// Package verificationsyn is the synthetic identity verifier of the test
// contour (the ingresssyn/verifier-synthetic precedent): it validates the
// eligibility of the method for the conversation modality and the kind over
// the domain dictionaries and passes the provider-observed code through.
// The real verifiers (AG-4) will supply their own provider codes through
// the same port — the canonical mapping stays the domain's; production
// contours without a wired verifier refuse honestly before any record.
package verificationsyn

import (
	"context"
	"fmt"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
)

// Verifier implements the cases IdentityVerifier port. The synthetic
// contour has no provider API: the operator observes the check outcome and
// the command carries the observed code — the adapter's honest job is the
// eligibility gate and the pass-through (the real adapters replace the
// pass-through with the provider answer, the domain mapping never moves).
type Verifier struct{}

// New wires the synthetic verifier.
func New() *Verifier { return &Verifier{} }

// Verify checks the eligibility of one verification attempt: the kind and
// the method coherence, the channel modality of the conversation and the
// provider code vocabulary. An ineligible attempt is an honest refusal with
// the allowed methods of the modality — never a silent "ok".
func (v *Verifier) Verify(ctx context.Context, request casesPort.IdentityVerificationRequest) (casesPort.IdentityVerification, error) {
	if casesDomain.MethodKind(request.Method) != request.Kind {
		return casesPort.IdentityVerification{}, fmt.Errorf("метод %s проверяет %s, а не %s", request.Method, casesDomain.MethodKind(request.Method), request.Kind)
	}
	if !casesDomain.MethodAllowedForModality(request.Method, request.Modality) {
		return casesPort.IdentityVerification{
			Eligible:          false,
			IneligibilityText: fmt.Sprintf("разрешённые методы модальности %s: %s", request.Modality, methodList(casesDomain.MethodsOfModality(request.Modality))),
			ProviderCode:      request.ProviderCode,
		}, nil
	}
	if request.Modality != casesDomain.ModalityVoice && request.Modality != casesDomain.ModalityText {
		return casesPort.IdentityVerification{
			Eligible:          false,
			IneligibilityText: fmt.Sprintf("модальность %s не входит в словарь каналов контура", request.Modality),
			ProviderCode:      request.ProviderCode,
		}, nil
	}
	return casesPort.IdentityVerification{Eligible: true, ProviderCode: request.ProviderCode}, nil
}

// methodList renders the methods for the refusal texts.
func methodList(methods []string) string {
	out := ""
	for i, method := range methods {
		if i > 0 {
			out += ", "
		}
		out += "«" + method + "»"
	}
	return out
}
