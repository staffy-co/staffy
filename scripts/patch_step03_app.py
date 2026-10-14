#!/usr/bin/env python3
# step-03 patch B: app/policy.go (4th input wiring), app/ownership.go (accuracy gate),
# app/service.go (CaseView accommodations), domain/ownership.go (ClientReply +AccuracyQualifier)
import io, sys

def patch(path, anchors):
    with io.open(path, encoding="utf-8") as f:
        text = f.read()
    for old, new in anchors:
        if text.count(old) != 1:
            print("FAIL anchor (count=%d) in %s:\n%r" % (text.count(old), path, old[:120]))
            sys.exit(1)
        text = text.replace(old, new)
    with io.open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    print("patched", path)

T = "\t"

# --- app/policy.go: EvaluatePolicy 4th input (the honest read before the transaction) ---
policy_old = (
    T + "// The 2.11 fourth input: the preferences state (nil keeps the control\n"
    + T + "// branch here — the honest wiring arrives with step-03; the store\n"
    + T + "// re-derives the decision under the locks anyway).\n"
    + T + "decision := casesDomain.EvaluatePolicy(*proposal, snapshot.world, casesDomain.EffectiveWorkModeWord(effective), nil)\n"
)
policy_new = (
    T + "// The 2.11 fourth input: the CURRENT preferences state of the case —\n"
    + T + "// the honest read before the transaction (the store re-derives the\n"
    + T + "// same truth under the SAME advisory locks; the snapshot carries the\n"
    + T + "// canonical preferences map — the digest determinism).\n"
    + T + "accommodationRecords, err := s.repository.ReadAccommodations(ctx, scope, command.CaseID)\n"
    + T + "if err != nil {\n"
    + T + T + "return casesAPI.WorkOutcome{}, err\n"
    + T + "}\n"
    + T + "currentAccommodations := casesDomain.DeriveCurrentAccommodations(accommodationRecords)\n"
    + T + "decision := casesDomain.EvaluatePolicy(*proposal, snapshot.world, casesDomain.EffectiveWorkModeWord(effective), currentAccommodations)\n"
)

patch("internal/cases/app/policy.go", [(policy_old, policy_new)])

# --- app/ownership.go: the request hash extension (the qualifier joins the material) ---
hash_old = (
    T + "// The request hash covers the whole command material INCLUDING the\n"
    + T + "// playbook step of the Story 2.8 extension and the presentation pair of\n"
    + T + "// the Story 2.9 extension (the same key with another material is the\n"
    + T + "// honest key conflict).\n"
    + T + "hash := requestHash(struct {\n"
    + T + T + "Case         string `json:\"case\"`\n"
    + T + T + "Conversation string `json:\"conversation\"`\n"
    + T + T + "WorkItem     string `json:\"work_item\"`\n"
    + T + T + "Expected     int64  `json:\"expected_epoch\"`\n"
    + T + T + "Class        string `json:\"reply_class\"`\n"
    + T + T + "Step         string `json:\"playbook_step\"`\n"
    + T + T + "Commitment   string `json:\"commitment_id\"`\n"
    + T + T + "CVersion     int64  `json:\"commitment_version\"`\n"
    + T + T + "Link         string `json:\"continuation_link_id\"`\n"
    + T + T + "Content      string `json:\"content\"`\n"
    + T + "}{command.CaseID.String(), command.ConversationID.String(), command.WorkItemID.String(), command.ExpectedEpoch, command.ReplyClass, command.PlaybookStep, command.CommitmentID.String(), command.CommitmentVersionNo, command.LinkID.String(), command.Content})\n"
)
hash_new = (
    T + "// The request hash covers the whole command material INCLUDING the\n"
    + T + "// playbook step of the Story 2.8 extension, the presentation pair of\n"
    + T + "// the Story 2.9 extension, the continuation binding of the Story 2.10\n"
    + T + "// extension and the accuracy qualifier word of the Story 2.11\n"
    + T + "// extension (the same key with another material is the honest key\n"
    + T + "// conflict).\n"
    + T + "hash := requestHash(struct {\n"
    + T + T + "Case         string `json:\"case\"`\n"
    + T + T + "Conversation string `json:\"conversation\"`\n"
    + T + T + "WorkItem     string `json:\"work_item\"`\n"
    + T + T + "Expected     int64  `json:\"expected_epoch\"`\n"
    + T + T + "Class        string `json:\"reply_class\"`\n"
    + T + T + "Step         string `json:\"playbook_step\"`\n"
    + T + T + "Commitment   string `json:\"commitment_id\"`\n"
    + T + T + "CVersion     int64  `json:\"commitment_version\"`\n"
    + T + T + "Link         string `json:\"continuation_link_id\"`\n"
    + T + T + "Qualifier    string `json:\"accuracy_qualifier\"`\n"
    + T + T + "Content      string `json:\"content\"`\n"
    + T + "}{command.CaseID.String(), command.ConversationID.String(), command.WorkItemID.String(), command.ExpectedEpoch, command.ReplyClass, command.PlaybookStep, command.CommitmentID.String(), command.CommitmentVersionNo, command.LinkID.String(), command.AccuracyQualifier, command.Content})\n"
)

# --- app/ownership.go: the accuracy gate AFTER the objection gate, BEFORE the commitment link ---
gate_old = (
    T + "objectionID := id.UUID(\"\")\n"
    + T + "if command.ReplyClass == casesDomain.ReplyClassSales {\n"
    + T + T + "objectionID, err = s.salesObjectionGate(ctx, scope, command.CaseID, command.WorkItemID, command.PlaybookStep)\n"
    + T + T + "if err != nil {\n"
    + T + T + T + "return casesAPI.WorkOutcome{}, err\n"
    + T + T + "}\n"
    + T + "}\n"
)
gate_new = (
    T + "objectionID := id.UUID(\"\")\n"
    + T + "if command.ReplyClass == casesDomain.ReplyClassSales {\n"
    + T + T + "objectionID, err = s.salesObjectionGate(ctx, scope, command.CaseID, command.WorkItemID, command.PlaybookStep)\n"
    + T + T + "if err != nil {\n"
    + T + T + T + "return casesAPI.WorkOutcome{}, err\n"
    + T + T + "}\n"
    + T + "}\n"
    + T + "// The accuracy gate (Story 2.11, AFTER the objection gate — the frozen\n"
    + T + "// ladder: ownership 2.4 → mode 2.5 → disclosure 2.5 → policy 2.6 →\n"
    + T + "// objection 2.8 → accuracy 2.11 → write; the SALES class only — the\n"
    + T + "// neutral and the protected paths stay byte-identical): the CURRENT\n"
    + T + "// outcome of the Work Item decides which words are honest — an\n"
    + T + "// outcome with active alternatives REQUIRES the qualifier word of its\n"
    + T + "// active set (the absence of media never becomes an invented exact\n"
    + T + "// result — the structural honesty of AC-2); an outcome without\n"
    + T + "// alternatives requires the EMPTY word (the field has exactly one\n"
    + T + "// meaning). The store re-validates under the locks.\n"
    + T + "if command.ReplyClass == casesDomain.ReplyClassSales {\n"
    + T + T + "if err := s.salesAccuracyGate(ctx, scope, command.CaseID, command.WorkItemID, command.AccuracyQualifier); err != nil {\n"
    + T + T + T + "return casesAPI.WorkOutcome{}, err\n"
    + T + T + "}\n"
    + T + "} else if command.AccuracyQualifier != \"\" {\n"
    + T + T + "return casesAPI.WorkOutcome{}, rule(\"send_client_reply\", fmt.Errorf(\"Слово ограничения точности несёт только коммерческий ответ (sales): нейтральные и защищённые сообщения не заявляют ограничение точности.\"))\n"
    + T + "}\n"
)

# --- app/ownership.go: the outcome carries the qualifier word ---
outcome_old = (
    T + "outcome := casesAPI.WorkOutcome{\n"
    + T + T + "Command: \"send_client_reply\", ReplyID: replyID,\n"
    + T + T + "ReplyState: \"prepared\", Epoch: current.Epoch, ReplyClass: command.ReplyClass,\n"
    + T + T + "PolicyOutcomeID: policyOutcomeID, ObjectionID: objectionID, PlaybookStep: command.PlaybookStep,\n"
    + T + "}\n"
)
outcome_new = (
    T + "outcome := casesAPI.WorkOutcome{\n"
    + T + T + "Command: \"send_client_reply\", ReplyID: replyID,\n"
    + T + T + "ReplyState: \"prepared\", Epoch: current.Epoch, ReplyClass: command.ReplyClass,\n"
    + T + T + "PolicyOutcomeID: policyOutcomeID, ObjectionID: objectionID, PlaybookStep: command.PlaybookStep,\n"
    + T + T + "AccuracyQualifier: command.AccuracyQualifier,\n"
    + T + "}\n"
)

# --- app/ownership.go: the WorkCommandParams extension ---
params_old = (
    T + T + "ObjectionID: objectionID, PlaybookStep: command.PlaybookStep,\n"
    + T + T + "CommitmentID: command.CommitmentID, VersionNo: command.CommitmentVersionNo,\n"
    + T + T + "LinkID:  command.LinkID,\n"
)
params_new = (
    T + T + "ObjectionID: objectionID, PlaybookStep: command.PlaybookStep,\n"
    + T + T + "CommitmentID: command.CommitmentID, VersionNo: command.CommitmentVersionNo,\n"
    + T + T + "LinkID:  command.LinkID,\n"
    + T + T + "AccuracyQualifier: command.AccuracyQualifier,\n"
)

patch("internal/cases/app/ownership.go", [
    (hash_old, hash_new),
    (gate_old, gate_new),
    (outcome_old, outcome_new),
    (params_old, params_new),
])

# --- app/service.go: CaseView assembles the accommodation section ---
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
svc_ret_old = (
    T + T + T + "Observations: observations, Objections: objections, Commitments: commitments,\n"
    + T + T + T + "Continuations: continuations,\n"
    + T + "}\n"
)
svc_ret_new = (
    T + T + T + "Observations: observations, Objections: objections, Commitments: commitments,\n"
    + T + T + T + "Continuations: continuations, Accommodations: accommodations,\n"
    + T + "}\n"
)

patch("internal/cases/app/service.go", [
    (svc_old, svc_new),
    (svc_ret_old, svc_ret_new),
])

# --- domain/ownership.go: ClientReply + AccuracyQualifier ---
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
