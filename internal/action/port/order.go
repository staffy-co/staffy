package port

import (
	"context"

	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// Story 4.6: the consumer-owned order version port. The dispatch ladder
// revalidates the order composition BEFORE the lease (AC-5): the port
// answers the LIVE derivation of the OrderDigest of one case — the same
// derivation the proposal builder produced (the adapter re-derives the
// material from the live case view and re-computes the one canonical
// digest; any substantial change of ANY of the five inputs changes the
// value).
//
// A NIL source is the HONEST production boundary (the FreshReads/
// OwningSystemReader precedent, AG-4): the drain of an order operation
// fails CLOSED with the exact word — the composition revalidation is
// never assumed «unchanged» without a live source.

// OrderVersionRequest is the read-only request of the live order
// composition digest: the tenant scope, the case anchor and the work item
// anchor of the operation being drained.
type OrderVersionRequest struct {
	Scope      tenancy.Context
	CaseID     id.UUID
	WorkItemID id.UUID
}

// OrderVersionSource is the consumer-owned port of the live order version:
// the adapter answers the canonical OrderDigest (64-hex SHA-256 of the
// canonical composition with the mandatory manager-check word) derived
// LIVE from the case view. ONE attempt per command: no retry, no cache —
// the revalidation reads the live truth or fails honestly.
type OrderVersionSource interface {
	// OrderCommitmentDigest derives the live canonical order digest of the
	// case (the same derivation the builder produced at the proposal time).
	OrderCommitmentDigest(ctx context.Context, request OrderVersionRequest) (string, error)
}
