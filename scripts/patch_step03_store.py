#!/usr/bin/env python3
# step-03 patch C: store/policy.go (4th input under the locks) + store/ownership.go
# (accuracy gate re-validation, accuracy_qualifier insert + read)
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

# --- store/policy.go: the fourth input under the SAME locks ---
policy_old = (
    T + "// The deterministic evaluation over the re-read material (the pure\n"
    + T + "// function — the store never trusts the app bytes).\n"
    + T + "// The 2.11 fourth input is the preferences state: the step-01 wiring\n"
    + T + "// keeps the control branch (nil — the decision stays byte-identical);\n"
    + T + "// the real read under the same advisory locks arrives with step-03.\n"
    + T + "decision := casesDomain.EvaluatePolicy(*proposal, world, effective, nil)\n"
)
policy_new = (
    T + "// The deterministic evaluation over the re-read material (the pure\n"
    + T + "// function — the store never trusts the app bytes).\n"
    + T + "// The 2.11 fourth input is the CURRENT preferences state: the honest\n"
    + T + "// read under the SAME advisory locks (the record command of the\n"
    + T + "// preferences serializes on the identical key set — the derivation\n"
    + T + "// is the lock-visible truth, never a raced snapshot).\n"
    + T + "accommodationRecords, err := readAccommodationsTx(ctx, tx, tenantID, p.CaseID)\n"
    + T + "if err != nil {\n"
    + T + T + "return err\n"
    + T + "}\n"
    + T + "currentAccommodations := casesDomain.DeriveCurrentAccommodations(accommodationRecords)\n"
    + T + "decision := casesDomain.EvaluatePolicy(*proposal, world, effective, currentAccommodations)\n"
)

# --- store/policy.go: the snapshot carries the canonical preferences map ---
snap_old = (
    T + T + "WorkItemVersion: current.Version, RuleVersion: casesDomain.PolicyRuleVersion,\n"
    + T + T + "Preferences: casesDomain.AccommodationSnapshotOf(nil),\n"
    + T + "}\n"
)
snap_new = (
    T + T + "WorkItemVersion: current.Version, RuleVersion: casesDomain.PolicyRuleVersion,\n"
    + T + T + "Preferences: casesDomain.AccommodationSnapshotOf(currentAccommodations),\n"
    + T + "}\n"
)

patch("internal/cases/store/policy.go", [
    (policy_old, policy_new),
    (snap_old, snap_new),
])

# --- store/ownership.go: the accuracy gate re-validation after the objection gate ---
gate_old = (
    T + "objectionID, err := replyObjectionGateOfTx(ctx, tx, tenantID, p.CaseID, p)\n"
    + T + "if err != nil {\n"
    + T + T + "return err\n"
    + T + "}\n"
)
gate_new = (
    T + "objectionID, err := replyObjectionGateOfTx(ctx, tx, tenantID, p.CaseID, p)\n"
    + T + "if err != nil {\n"
    + T + T + "return err\n"
    + T + "}\n"
    + T + "// The accuracy gate re-validation (Story 2.11, AFTER the objection gate\n"
    + T + "// — the frozen ladder: ... → objection 2.8 → accuracy 2.11 → write;\n"
    + T + "// the SALES class only — the neutral and the protected paths stay\n"
    + T + "// byte-identical): the CURRENT outcome of the Work Item decides the\n"
    + T + "// honest words — active alternatives REQUIRE the qualifier of the\n"
    + T + "// active set; no alternatives demand the empty word (the app validated\n"
    + T + "// pre-transaction; the store re-validates the server-side truth under\n"
    + T + "// the locks).\n"
    + T + "if err := replyAccuracyGateOfTx(ctx, tx, tenantID, p.CaseID, p); err != nil {\n"
    + T + T + "return err\n"
    + T + "}\n"
)

# --- store/ownership.go: the reply insert carries the qualifier word ---
insert_old = (
    T + "if _, err = tx.Exec(ctx, `\n"
    + T + T + "insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version, content, actor_id, reply_class, work_item_id, policy_outcome_id, objection_id, playbook_step, commitment_id, commitment_version, link_id, created_at)\n"
    + T + T + "values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)`,\n"
    + T + T + "tenantID, p.CaseID, p.ConversationID, p.ReplyID, current.OwnerKind, current.OwnerRef,\n"
    + T + T + "current.Epoch, current.WritableSurface, conversation.ConnectionID, conversation.Provider,\n"
    + T + T + "channel, \"outbound\", p.LineageEmployeeID, p.LineageDecisionNo, p.LineageProfileVersion,\n"
    + T + T + "p.ReplyContent, p.Actor, nilText(p.ReplyClass), nilID(p.WorkItemID), nilID(policyOutcomeID), nilID(objectionID), nilText(p.PlaybookStep), nilID(p.CommitmentID), nilInt64(p.VersionNo), nilID(p.LinkID), now); err != nil {\n"
    + T + T + "return mapOwnershipInsertError(err)\n"
    + T + "}\n"
)
insert_new = (
    T + "if _, err = tx.Exec(ctx, `\n"
    + T + T + "insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version, content, actor_id, reply_class, work_item_id, policy_outcome_id, objection_id, playbook_step, commitment_id, commitment_version, link_id, accuracy_qualifier, created_at)\n"
    + T + T + "values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27)`,\n"
    + T + T + "tenantID, p.CaseID, p.ConversationID, p.ReplyID, current.OwnerKind, current.OwnerRef,\n"
    + T + T + "current.Epoch, current.WritableSurface, conversation.ConnectionID, conversation.Provider,\n"
    + T + T + "channel, \"outbound\", p.LineageEmployeeID, p.LineageDecisionNo, p.LineageProfileVersion,\n"
    + T + T + "p.ReplyContent, p.Actor, nilText(p.ReplyClass), nilID(p.WorkItemID), nilID(policyOutcomeID), nilID(objectionID), nilText(p.PlaybookStep), nilID(p.CommitmentID), nilInt64(p.VersionNo), nilID(p.LinkID), nilText(p.AccuracyQualifier), now); err != nil {\n"
    + T + T + "return mapOwnershipInsertError(err)\n"
    + T + "}\n"
)

# --- store/ownership.go: the reply read carries the qualifier word (the panel truth) ---
read_old = (
    T + "rows, err := tx.Query(ctx, `\n"
    + T + T + "select r.reply_id, r.case_id, r.conversation_id, r.owner_kind, r.owner_ref, r.epoch, r.writable_surface,\n"
    + T + T + T + T + "r.connection_id, r.provider, r.channel, r.direction, r.employee_id, r.decision_no, r.profile_version,\n"
    + T + T + T + T + "r.content, r.actor_id, r.created_at, o.kind, o.created_at\n"
    + T + T + "from cases.client_replies r\n"
    + T + T + "left join cases.client_reply_outcomes o on o.tenant_id = r.tenant_id and o.reply_id = r.reply_id\n"
    + T + T + "where r.tenant_id=$1 and r.case_id=$2 order by r.created_at, r.reply_id`,\n"
    + T + T + "scope.TenantID(), caseID)\n"
)
read_new = (
    T + "rows, err := tx.Query(ctx, `\n"
    + T + T + "select r.reply_id, r.case_id, r.conversation_id, r.owner_kind, r.owner_ref, r.epoch, r.writable_surface,\n"
    + T + T + T + T + "r.connection_id, r.provider, r.channel, r.direction, r.employee_id, r.decision_no, r.profile_version,\n"
    + T + T + T + T + "r.content, r.accuracy_qualifier, r.actor_id, r.created_at, o.kind, o.created_at\n"
    + T + T + "from cases.client_replies r\n"
    + T + T + "left join cases.client_reply_outcomes o on o.tenant_id = r.tenant_id and o.reply_id = r.reply_id\n"
    + T + T + "where r.tenant_id=$1 and r.case_id=$2 order by r.created_at, r.reply_id`,\n"
    + T + T + "scope.TenantID(), caseID)\n"
)
scan_old = (
    T + T + T + "if err = rows.Scan(&reply.ReplyID, &reply.CaseID, &reply.ConversationID, &reply.OwnerKind, &reply.OwnerRef,\n"
    + T + T + T + T + T + "&reply.Epoch, &reply.WritableSurface, &reply.ConnectionID, &reply.Provider, &reply.Channel,\n"
    + T + T + T + T + T + "&reply.Direction, &reply.EmployeeID, &reply.DecisionNo, &reply.ProfileVersion, &reply.Content,\n"
    + T + T + T + T + T + "&reply.ActorID, &reply.CreatedAt, &outcomeKind, &outcomeAt); err != nil {\n"
)
scan_new = (
    T + T + T + "if err = rows.Scan(&reply.ReplyID, &reply.CaseID, &reply.ConversationID, &reply.OwnerKind, &reply.OwnerRef,\n"
    + T + T + T + T + T + "&reply.Epoch, &reply.WritableSurface, &reply.ConnectionID, &reply.Provider, &reply.Channel,\n"
    + T + T + T + T + T + "&reply.Direction, &reply.EmployeeID, &reply.DecisionNo, &reply.ProfileVersion, &reply.Content,\n"
    + T + T + T + T + T + "&reply.AccuracyQualifier, &reply.ActorID, &reply.CreatedAt, &outcomeKind, &outcomeAt); err != nil {\n"
)

patch("internal/cases/store/ownership.go", [
    (gate_old, gate_new),
    (insert_old, insert_new),
    (read_old, read_new),
    (scan_old, scan_new),
])

print("ALL PATCHES OK")
