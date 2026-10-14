#!/usr/bin/env python3
# step-03 patch C2: store/ownership.go remaining anchors (SQL lines use spaces, Go args use tabs)
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
S16 = " " * 16
S23 = " " * 23

# --- the reply insert carries the qualifier word (SQL on spaces, args on tabs) ---
insert_old = (
    T + "if _, err = tx.Exec(ctx, `\n"
    + S16 + "insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version, content, actor_id, reply_class, work_item_id, policy_outcome_id, objection_id, playbook_step, commitment_id, commitment_version, link_id, created_at)\n"
    + S16 + "values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)`,\n"
    + T + T + "tenantID, p.CaseID, p.ConversationID, p.ReplyID, current.OwnerKind, current.OwnerRef,\n"
    + T + T + "current.Epoch, current.WritableSurface, conversation.ConnectionID, conversation.Provider,\n"
    + T + T + "channel, \"outbound\", p.LineageEmployeeID, p.LineageDecisionNo, p.LineageProfileVersion,\n"
    + T + T + "p.ReplyContent, p.Actor, nilText(p.ReplyClass), nilID(p.WorkItemID), nilID(policyOutcomeID), nilID(objectionID), nilText(p.PlaybookStep), nilID(p.CommitmentID), nilInt64(p.VersionNo), nilID(p.LinkID), now); err != nil {\n"
)
insert_new = (
    T + "if _, err = tx.Exec(ctx, `\n"
    + S16 + "insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version, content, actor_id, reply_class, work_item_id, policy_outcome_id, objection_id, playbook_step, commitment_id, commitment_version, link_id, accuracy_qualifier, created_at)\n"
    + S16 + "values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27)`,\n"
    + T + T + "tenantID, p.CaseID, p.ConversationID, p.ReplyID, current.OwnerKind, current.OwnerRef,\n"
    + T + T + "current.Epoch, current.WritableSurface, conversation.ConnectionID, conversation.Provider,\n"
    + T + T + "channel, \"outbound\", p.LineageEmployeeID, p.LineageDecisionNo, p.LineageProfileVersion,\n"
    + T + T + "p.ReplyContent, p.Actor, nilText(p.ReplyClass), nilID(p.WorkItemID), nilID(policyOutcomeID), nilID(objectionID), nilText(p.PlaybookStep), nilID(p.CommitmentID), nilInt64(p.VersionNo), nilID(p.LinkID), nilText(p.AccuracyQualifier), now); err != nil {\n"
)

# --- the reply read carries the qualifier word (SQL on spaces) ---
read_old = (
    T + "rows, err := tx.Query(ctx, `\n"
    + S16 + "select r.reply_id, r.case_id, r.conversation_id, r.owner_kind, r.owner_ref, r.epoch, r.writable_surface,\n"
    + S23 + "r.connection_id, r.provider, r.channel, r.direction, r.employee_id, r.decision_no, r.profile_version,\n"
    + S23 + "r.content, r.actor_id, r.created_at, o.kind, o.created_at\n"
    + S16 + "from cases.client_replies r\n"
    + S16 + "left join cases.client_reply_outcomes o on o.tenant_id = r.tenant_id and o.reply_id = r.reply_id\n"
    + S16 + "where r.tenant_id=$1 and r.case_id=$2 order by r.created_at, r.reply_id`,\n"
    + T + T + "scope.TenantID(), caseID)\n"
)
read_new = (
    T + "rows, err := tx.Query(ctx, `\n"
    + S16 + "select r.reply_id, r.case_id, r.conversation_id, r.owner_kind, r.owner_ref, r.epoch, r.writable_surface,\n"
    + S23 + "r.connection_id, r.provider, r.channel, r.direction, r.employee_id, r.decision_no, r.profile_version,\n"
    + S23 + "r.content, r.accuracy_qualifier, r.actor_id, r.created_at, o.kind, o.created_at\n"
    + S16 + "from cases.client_replies r\n"
    + S16 + "left join cases.client_reply_outcomes o on o.tenant_id = r.tenant_id and o.reply_id = r.reply_id\n"
    + S16 + "where r.tenant_id=$1 and r.case_id=$2 order by r.created_at, r.reply_id`,\n"
    + T + T + "scope.TenantID(), caseID)\n"
)

# --- the scan reads the qualifier (Go tabs) ---
scan_old = (
    T + T + "if err = rows.Scan(&reply.ReplyID, &reply.CaseID, &reply.ConversationID, &reply.OwnerKind, &reply.OwnerRef,\n"
    + T + T + T + "&reply.Epoch, &reply.WritableSurface, &reply.ConnectionID, &reply.Provider, &reply.Channel,\n"
    + T + T + T + "&reply.Direction, &reply.EmployeeID, &reply.DecisionNo, &reply.ProfileVersion, &reply.Content,\n"
    + T + T + T + "&reply.ActorID, &reply.CreatedAt, &outcomeKind, &outcomeAt); err != nil {\n"
)
scan_new = (
    T + T + "if err = rows.Scan(&reply.ReplyID, &reply.CaseID, &reply.ConversationID, &reply.OwnerKind, &reply.OwnerRef,\n"
    + T + T + T + "&reply.Epoch, &reply.WritableSurface, &reply.ConnectionID, &reply.Provider, &reply.Channel,\n"
    + T + T + T + "&reply.Direction, &reply.EmployeeID, &reply.DecisionNo, &reply.ProfileVersion, &reply.Content,\n"
    + T + T + T + "&reply.AccuracyQualifier, &reply.ActorID, &reply.CreatedAt, &outcomeKind, &outcomeAt); err != nil {\n"
)

# --- the accuracy gate re-validation after the objection gate (Go tabs, statement depth 1) ---
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

patch("internal/cases/store/ownership.go", [
    (insert_old, insert_new),
    (read_old, read_new),
    (scan_old, scan_new),
    (gate_old, gate_new),
])

print("ALL PATCHES OK")
