#!/usr/bin/env python3
# step-03 patch B2: app/service.go (CaseView accommodations return) + domain/ownership.go
import io, sys

def patch(path, anchors):
    with io.open(path, encoding="utf-8") as f:
        text = f.read()
    for old, new in anchors:
        if text.count(old) != 1:
            if new in text:
                print("SKIP (already patched)", path)
                continue
            print("FAIL anchor (count=%d) in %s:\n%r" % (text.count(old), path, old[:120]))
            sys.exit(1)
        text = text.replace(old, new)
    with io.open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    print("patched", path)

T = "\t"

# --- app/service.go: CaseView assembles the accommodation section (statement depth = 1 tab) ---
svc_old = (
    T + "// The Story 2.10 section: the continuation links of the case with the\n"
    + T + "// read-time state derivations, the delivered binding of the outgoing\n"
    + T + "// replies, the use accounting against the limit and the event feed (the\n"
    + T + "// reads degrade honestly — the same view scope).\n"
    + T + "continuations, err := s.continuationView(ctx, scope, detail)\n"
    + T + "if err != nil {\n"
    + T + T + "return casesAPI.CaseView{}, err\n"
    + T + "}\n"
    + T + "return casesAPI.CaseView{\n"
)
svc_new = (
    T + "// The Story 2.10 section: the continuation links of the case with the\n"
    + T + "// read-time state derivations, the delivered binding of the outgoing\n"
    + T + "// replies, the use accounting against the limit and the event feed (the\n"
    + T + "// reads degrade honestly — the same view scope).\n"
    + T + "continuations, err := s.continuationView(ctx, scope, detail)\n"
    + T + "if err != nil {\n"
    + T + T + "return casesAPI.CaseView{}, err\n"
    + T + "}\n"
    + T + "// The Story 2.11 section: the recorded communication preferences — the\n"
    + T + "// CURRENT set (the read-time derivation), the append-only revision\n"
    + T + "// history and the dictionaries of the record form (the reads degrade\n"
    + T + "// honestly — the same view scope).\n"
    + T + "accommodations, err := s.accommodationView(ctx, scope, detail)\n"
    + T + "if err != nil {\n"
    + T + T + "return casesAPI.CaseView{}, err\n"
    + T + "}\n"
    + T + "return casesAPI.CaseView{\n"
)

# --- app/service.go: the CaseView return literal (field depth = 2 tabs) ---
svc_ret_old = (
    T + T + "Observations: observations, Objections: objections, Commitments: commitments,\n"
    + T + T + "Continuations: continuations,\n"
    + T + "}, nil\n"
)
svc_ret_new = (
    T + T + "Observations: observations, Objections: objections, Commitments: commitments,\n"
    + T + T + "Continuations: continuations, Accommodations: accommodations,\n"
    + T + "}, nil\n"
)

patch("internal/cases/app/service.go", [
    (svc_old, svc_new),
    (svc_ret_old, svc_ret_new),
])

# --- domain/ownership.go: ClientReply + AccuracyQualifier (field depth = 1 tab) ---
dom_old = (
    T + "// The Story 2.6 material: the Policy Outcome the sales reply passed\n"
    + T + "// through (the zero id of the pre-migration and non-sales rows).\n"
    + T + "PolicyOutcomeID id.UUID\n"
    + T + "CreatedAt       time.Time\n"
    + "}\n"
)
dom_new = (
    T + "// The Story 2.6 material: the Policy Outcome the sales reply passed\n"
    + T + "// through (the zero id of the pre-migration and non-sales rows).\n"
    + T + "PolicyOutcomeID id.UUID\n"
    + T + "// The Story 2.11 material: the accuracy qualifier word of the sales\n"
    + T + "// reply against an outcome with active alternatives (the honest\n"
    + T + "// limitation word of the outcome's active set; empty when the outcome\n"
    + T + "// carries none — the field has exactly one meaning).\n"
    + T + "AccuracyQualifier string\n"
    + T + "CreatedAt       time.Time\n"
    + "}\n"
)

patch("internal/cases/domain/ownership.go", [(dom_old, dom_new)])

print("ALL PATCHES OK")
