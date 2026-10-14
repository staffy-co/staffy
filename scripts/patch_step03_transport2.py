#!/usr/bin/env python3
# step-03 patch D2: transport routes (cases.go, 1-tab) + Server field + ownership pass-through (fixed depths)
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

# --- cases.go: the Story 2.11 routes (1-tab statement, 2-tab body) ---
routes_old = (
    T + "if continuations, ok := service.(casesAPI.ContinuationService); ok {\n"
    + T + T + "s.continuations = continuations\n"
)
routes_new = (
    T + "if accommodations, ok := service.(casesAPI.AccommodationService); ok {\n"
    + T + T + "// The Story 2.11 surfaces: the accommodation record command (machine\n"
    + T + T + "// JSON + the operator PRG form of the «Предпочтения связи» panel — the\n"
    + T + T + "// narrow interface is type-asserted of the cases service (the\n"
    + T + T + "// ContinuationService precedent: the composite Service and the test\n"
    + T + T + "// fakes stay untouched).\n"
    + T + T + "s.accommodations = accommodations\n"
    + T + T + "s.mux.HandleFunc(\"POST /v1/tenants/{tenant_id}/cases/{case_id}/accommodations\", s.recordAccommodationAPI)\n"
    + T + T + "s.mux.HandleFunc(\"POST /app/tenants/{tenant_id}/cases/{case_id}/accommodations\", s.recordAccommodationForm)\n"
    + T + "}\n"
    + T + "if continuations, ok := service.(casesAPI.ContinuationService); ok {\n"
    + T + T + "s.continuations = continuations\n"
)

patch("internal/transport/http/cases.go", [(routes_old, routes_new)])

# --- server.go: the accommodations service field ---
S8 = " " * 8
server_old = (
    S8 + "cases     casesAPI.Service\n"
    + S8 + "continuations casesAPI.ContinuationService\n"
)
server_new = (
    S8 + "cases     casesAPI.Service\n"
    + S8 + "accommodations casesAPI.AccommodationService\n"
    + S8 + "continuations casesAPI.ContinuationService\n"
)

patch("internal/transport/http/server.go", [(server_old, server_new)])

# --- cases_ownership.go: the input struct + the qualifier field (1-tab fields) ---
input_old = (
    T + "// LinkID is the optional Story 2.10 delivery binding: the SALES\n"
    + T + "// reply carries ONE continuation link of the case whose token-URL\n"
    + T + "// the operator wove into the text (the delivered badge of the FR-16);\n"
    + T + "// the neutral and the protected classes never carry it.\n"
    + T + "LinkID *string `json:\"link_id\"`\n"
    + "}\n"
)
input_new = (
    T + "// LinkID is the optional Story 2.10 delivery binding: the SALES\n"
    + T + "// reply carries ONE continuation link of the case whose token-URL\n"
    + T + "// the operator wove into the text (the delivered badge of the FR-16);\n"
    + T + "// the neutral and the protected classes never carry it.\n"
    + T + "LinkID *string `json:\"link_id\"`\n"
    + T + "// AccuracyQualifier is the optional Story 2.11 honesty word: the\n"
    + T + "// SALES reply against an outcome with active alternatives MUST carry\n"
    + T + "// the qualifier word of the outcome's active set (a subset of the\n"
    + T + "// OUTCOME, never the dictionary in general); without alternatives\n"
    + T + "// the word stays empty — the field has exactly one meaning.\n"
    + T + "AccuracyQualifier *string `json:\"accuracy_qualifier\"`\n"
    + "}\n"
)

# --- cases_ownership.go: the machine JSON carries the qualifier (1-tab stmt, 2-tab fields) ---
api_old = (
    T + "outcome, err := s.cases.SendClientReply(r.Context(), casesAPI.SendClientReplyCommand{\n"
    + T + T + "Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,\n"
    + T + T + "WorkItemID:    workItemID,\n"
    + T + T + "ExpectedEpoch: *input.ExpectedEpoch,\n"
    + T + T + "ReplyClass:    stringOrEmpty(input.ReplyClass),\n"
    + T + T + "PlaybookStep:  stringOrEmpty(input.PlaybookStep),\n"
    + T + T + "LinkID:        linkID,\n"
    + T + T + "Content:       stringOrEmpty(input.Content), IdempotencyKey: key, Now: s.clock.Now().UTC(),\n"
    + T + "})\n"
)
api_new = (
    T + "outcome, err := s.cases.SendClientReply(r.Context(), casesAPI.SendClientReplyCommand{\n"
    + T + T + "Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,\n"
    + T + T + "WorkItemID:    workItemID,\n"
    + T + T + "ExpectedEpoch: *input.ExpectedEpoch,\n"
    + T + T + "ReplyClass:    stringOrEmpty(input.ReplyClass),\n"
    + T + T + "PlaybookStep:  stringOrEmpty(input.PlaybookStep),\n"
    + T + T + "LinkID:        linkID,\n"
    + T + T + "AccuracyQualifier: stringOrEmpty(input.AccuracyQualifier),\n"
    + T + T + "Content:       stringOrEmpty(input.Content), IdempotencyKey: key, Now: s.clock.Now().UTC(),\n"
    + T + "})\n"
)

# --- cases_ownership.go: the form passes the qualifier word (1-tab stmt, 2-tab fields) ---
form_old = (
    T + "_, err = s.cases.SendClientReply(r.Context(), casesAPI.SendClientReplyCommand{\n"
    + T + T + "Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,\n"
    + T + T + "WorkItemID:    workItemID,\n"
    + T + T + "ExpectedEpoch: expected,\n"
    + T + T + "ReplyClass:    replyClass,\n"
    + T + T + "PlaybookStep:  playbookStep,\n"
    + T + T + "CommitmentID:  commitmentID, CommitmentVersionNo: commitmentVersion,\n"
    + T + T + "LinkID:         linkID,\n"
    + T + T + "Content:        content,\n"
    + T + T + "IdempotencyKey: formCommandKey(\"send-client-reply\", actor.ID.String(), caseID.String(), conversationID.String(), workItemID.String(), strconv.FormatInt(expected, 10), replyClass, playbookStep, commitmentID.String(), strconv.FormatInt(commitmentVersion, 10), linkID.String(), content),\n"
    + T + T + "Now:            s.clock.Now().UTC(),\n"
    + T + "})\n"
)
form_new = (
    T + "// The Story 2.11 accuracy qualifier word: the optional select of the\n"
    + T + "// form (the SALES reply against an outcome with active alternatives\n"
    + T + "// MUST carry it — the refusal names the honest rule and the active\n"
    + T + "// words; the select carries NO preselected option: the operator\n"
    + T + "// chooses the limitation consciously).\n"
    + T + "accuracyQualifier := r.FormValue(\"accuracy_qualifier\")\n"
    + T + "_, err = s.cases.SendClientReply(r.Context(), casesAPI.SendClientReplyCommand{\n"
    + T + T + "Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,\n"
    + T + T + "WorkItemID:    workItemID,\n"
    + T + T + "ExpectedEpoch: expected,\n"
    + T + T + "ReplyClass:    replyClass,\n"
    + T + T + "PlaybookStep:  playbookStep,\n"
    + T + T + "CommitmentID:  commitmentID, CommitmentVersionNo: commitmentVersion,\n"
    + T + T + "LinkID:         linkID,\n"
    + T + T + "AccuracyQualifier: accuracyQualifier,\n"
    + T + T + "Content:        content,\n"
    + T + T + "IdempotencyKey: formCommandKey(\"send-client-reply\", actor.ID.String(), caseID.String(), conversationID.String(), workItemID.String(), strconv.FormatInt(expected, 10), replyClass, playbookStep, accuracyQualifier, commitmentID.String(), strconv.FormatInt(commitmentVersion, 10), linkID.String(), content),\n"
    + T + T + "Now:            s.clock.Now().UTC(),\n"
    + T + "})\n"
)

patch("internal/transport/http/cases_ownership.go", [
    (input_old, input_new),
    (api_old, api_new),
    (form_old, form_new),
])

print("ALL PATCHES OK")
