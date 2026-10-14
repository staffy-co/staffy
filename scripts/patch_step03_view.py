#!/usr/bin/env python3
# step-03 patch E: cases_view.go — the accommodation panel, the accuracy
# options, the transfer card hint, the reply qualifier word, the policy
# alternatives rendering, the continuation form hint
import io, sys

def patch(path, anchors):
    with io.open(path, encoding="utf-8") as f:
        text = f.read()
    for old, new in anchors:
        if text.count(old) != 1:
            if new in text:
                print("SKIP (already patched)", path)
                continue
            print("FAIL anchor (count=%d) in %s:\n%r" % (text.count(old), path, old[:150]))
            sys.exit(1)
        text = text.replace(old, new)
    with io.open(path, "w", encoding="utf-8", newline="") as f:
        f.write(text)
    print("patched", path)

T = "\t"

# 1. caseDetailPageView: the Story 2.11 fields (after the Continuation section) ---
cv_old = (
    T + "ContinuationsPanelText     string\n"
    + T + "ContinuationLinkOptions    []workOption\n"
    + T + "ContinuationPurposeOptions []workOption\n"
    + T + "ContinuationRevoked        bool\n"
    + "}\n"
)
cv_new = (
    T + "ContinuationsPanelText     string\n"
    + T + "ContinuationLinkOptions    []workOption\n"
    + T + "ContinuationPurposeOptions []workOption\n"
    + T + "ContinuationRevoked        bool\n"
    + T + "// The Story 2.11 section: the accommodation panel words (the CURRENT\n"
    + T + "// set, the append-only revision history, the key options of the record\n"
    + T + "// form — the dictionaries are fixed, never invented options), the\n"
    + T + "// accuracy qualifier options of the sales reply form (the active\n"
    + T + "// alternative words of the CURRENT outcomes: the form hint — the\n"
    + T + "// server gate resolves the exact set of the selected Work Item) and\n"
    + T + "// the continuation issue form hint (the honest no_web/no_voice words —\n"
    + T + "// the operator decides the surface, the machine never blocks).\n"
    + T + "AccommodationsPanelText    string\n"
    + T + "AccommodationCurrentRows   []accommodationStateRow\n"
    + T + "AccommodationHistoryRows   []accommodationRecordRow\n"
    + T + "HasAccommodations          bool\n"
    + T + "AccommodationKeyOptions    []workOption\n"
    + T + "AccommodationRecordsAnchor string\n"
    + T + "AccuracyQualifierOptions   []workOption\n"
    + T + "ContinuationSurfaceHint    string\n"
    + "}\n"
)

# 2. replyRowView: the qualifier word of the honest limitation ---
reply_old = (
    T + "Epoch     string\n"
    + T + "OwnerText string\n"
    + "}\n"
)
reply_new = (
    T + "Epoch     string\n"
    + T + "OwnerText string\n"
    + T + "// QualifierText is the accuracy qualifier word of the Story 2.11 reply\n"
    + T + "// (the honest limitation of the outcome's active set; empty when the\n"
    + T + "// outcome carried none).\n"
    + T + "QualifierText string\n"
    + "}\n"
)

# 3. ownershipPanelView: the transfer card preference words ---
own_old = (
    T + "HasReplies       bool\n"
    + T + "ReplyRows        []replyRowView\n"
    + T + "MemberOptions    []workOption\n"
    + "}\n"
)
own_new = (
    T + "HasReplies       bool\n"
    + T + "ReplyRows        []replyRowView\n"
    + T + "MemberOptions    []workOption\n"
    + T + "// AccommodationWords is the current preference words of the Story 2.11\n"
    + T + "// transfer card: the receiving operator reads the client's\n"
    + T + "// communication preferences BEFORE the ack (AC-1 «передаются каждому\n"
    + T + "// последующему Response Owner»).\n"
    + T + "AccommodationWords string\n"
    + "}\n"
)

# 4. prepareCaseView: the Story 2.11 section assembly (after the Commitments block) ---
prep_old = (
    T + "out.CommitmentsPanelText = view.Commitments.PanelText\n"
    + T + "for _, section := range view.Commitments.Items {\n"
    + T + T + "for _, card := range section.Versions {\n"
    + T + T + T + "if card.Current && card.Version.Status == casesDomain.CommitmentStatusOffered {\n"
    + T + T + T + T + "out.CommitmentLinkOptions = append(out.CommitmentLinkOptions, workOption{\n"
    + T + T + T + T + T + "Value: card.Version.CommitmentID.String() + \":\" + strconv.FormatInt(card.Version.VersionNo, 10),\n"
    + T + T + T + T + T + "Text:  \"обязательство \" + card.Version.CommitmentID.String() + \" · версия №\" + strconv.FormatInt(card.Version.VersionNo, 10) + \" (offered)\",\n"
    + T + T + T + T + "})\n"
    + T + T + T + "}\n"
    + T + T + "}\n"
    + T + "}\n"
)
prep_new = prep_old + (
    T + "\n"
    + T + "// The Story 2.11 section: the accommodation panel — the CURRENT set\n"
    + T + "// (the read-time derivation: the cleared keys stay visible with the\n"
    + T + "// honest mark), the append-only revision history, the key options of\n"
    + T + "// the record form (the fixed dictionary) and the accuracy qualifier\n"
    + T + "// options of the sales reply form (the active words of the CURRENT\n"
    + T + "// outcomes — the union over the case items; the server gate resolves\n"
    + T + "// the exact set of the selected item, the form shows the hint).\n"
    + T + "out.AccommodationsPanelText = view.Accommodations.PanelText\n"
    + T + "out.HasAccommodations = len(view.Accommodations.History) > 0\n"
    + T + "out.AccommodationRecordsAnchor = strconv.Itoa(view.Accommodations.RecordsCount)\n"
    + T + "for _, state := range view.Accommodations.Current {\n"
    + T + T + "out.AccommodationCurrentRows = append(out.AccommodationCurrentRows, accommodationStateRow{\n"
    + T + T + T + "Key: state.Key, KeyText: accommodationKeyText(state.Key),\n"
    + T + T + T + "Revision: strconv.FormatInt(state.Revision, 10), Cleared: state.Cleared, Value: state.Value,\n"
    + T + T + "})\n"
    + T + "}\n"
    + T + "for _, record := range view.Accommodations.History {\n"
    + T + T + "out.AccommodationHistoryRows = append(out.AccommodationHistoryRows, accommodationRecordRow{\n"
    + T + T + T + "Key: record.Key, KeyText: accommodationKeyText(record.Key),\n"
    + T + T + T + "Revision: strconv.FormatInt(record.Revision, 10), Value: record.Value, Cleared: record.Cleared,\n"
    + T + T + T + "Note: record.Note, ActorID: record.ActorID.String(), RecordedAt: formatTime(record.RecordedAt),\n"
    + T + T + "})\n"
    + T + "}\n"
    + T + "for _, word := range view.Accommodations.KeyWords {\n"
    + T + T + "out.AccommodationKeyOptions = append(out.AccommodationKeyOptions, workOption{Value: word, Text: accommodationKeyText(word)})\n"
    + T + "}\n"
    + T + "qualifierWords := map[string]bool{}\n"
    + T + "for _, section := range view.Policy.Items {\n"
    + T + T + "if section.Current != nil {\n"
    + T + T + T + "for _, word := range section.Current.ActiveAlternatives {\n"
    + T + T + T + T + "qualifierWords[word] = true\n"
    + T + T + T + "}\n"
    + T + T + "}\n"
    + T + "}\n"
    + T + "for _, word := range casesDomain.AlternativePathWords {\n"
    + T + T + "if qualifierWords[word] {\n"
    + T + T + T + "out.AccuracyQualifierOptions = append(out.AccuracyQualifierOptions, workOption{Value: word, Text: accuracyQualifierText(word)})\n"
    + T + T + "}\n"
    + T + "}\n"
    + T + "// The continuation issue form hint: the honest no_web/no_voice words of\n"
    + T + "// the current set (the link IS a web page — the operator decides the\n"
    + T + "// surface; the machine never blocks without a capability registry).\n"
    + T + "surfaceHints := []string{}\n"
    + T + "for _, state := range view.Accommodations.Current {\n"
    + T + T + "if !state.Cleared && (state.Key == casesDomain.AccommodationNoWeb || state.Key == casesDomain.AccommodationNoVoice) {\n"
    + T + T + T + "surfaceHints = append(surfaceHints, accommodationKeyText(state.Key))\n"
    + T + T + "}\n"
    + T + "}\n"
    + T + "if len(surfaceHints) > 0 {\n"
    + T + T + "out.ContinuationSurfaceHint = \"Записаны предпочтения клиента: \" + strings.Join(surfaceHints, \"; \") + \". Ссылка продолжения — web-страница: сверьте, подходит ли эта поверхность клиенту (машина не блокирует — решение за оператором).\"\n"
    + T + "}\n"
)

# 5. policyOutcomeRow: the active alternatives of the outcome ---
pol_old = (
    T + "type policyOutcomeRow struct {\n"
    + T + "OutcomeID      string\n"
)
pol_new = (
    T + "type policyOutcomeRow struct {\n"
    + T + "OutcomeID      string\n"
)
# (the fields anchor below inserts Alternatives before PermittedText)
pol_fields_old = (
    T + "Suppressed     []string\n"
    + T + "PermittedText  string\n"
    + T + "DecisionDigest string\n"
    + T + "CreatedAt      string\n"
    + "}\n"
)
pol_fields_new = (
    T + "Suppressed     []string\n"
    + T + "Alternatives   []policyAlternativeRow\n"
    + T + "PermittedText   string\n"
    + T + "DecisionDigest  string\n"
    + T + "CreatedAt      string\n"
    + "}\n\n"
    + "// policyAlternativeRow is one active alternative word of the outcome:\n"
    + "// the word with its honest limitation text (the dictionary of the\n"
    + "// Domain Pack — the panel renders the text, the database carries the\n"
    + "// words only).\n"
    + "type policyAlternativeRow struct {\n"
    + T + "Word           string\n"
    + T + "WordText       string\n"
    + T + "LimitationText string\n"
    + "}\n"
)

# 6. preparePolicyOutcomeRow: render the alternatives ---
ppr_old = (
    T + "row := policyOutcomeRow{\n"
    + T + T + "OutcomeID:      outcome.OutcomeID.String(),\n"
)
ppr_new = (
    T + "row := policyOutcomeRow{\n"
    + T + T + "OutcomeID:      outcome.OutcomeID.String(),\n"
)
ppr_fields_old = (
    T + T + "Suppressed:     make([]string, 0, len(outcome.SuppressedQuestions)),\n"
    + T + T + "PermittedText:  \"Разрешённый путь: \" + casesDomain.OutcomePermittedPath(outcome.OutcomeClass),\n"
)
ppr_fields_new = (
    T + T + "Suppressed:     make([]string, 0, len(outcome.SuppressedQuestions)),\n"
    + T + T + "Alternatives:   make([]policyAlternativeRow, 0, len(outcome.ActiveAlternatives)),\n"
    + T + T + "PermittedText:  \"Разрешённый путь: \" + casesDomain.OutcomePermittedPath(outcome.OutcomeClass),\n"
)
ppr_loop_old = (
    T + "for _, suppressed := range outcome.SuppressedQuestions {\n"
    + T + T + "row.Suppressed = append(row.Suppressed, suppressed.MaterialKey+\" (источник: \"+suppressed.Source+\")\")\n"
    + T + "}\n"
    + T + "return row\n"
)
ppr_loop_new = (
    T + "for _, suppressed := range outcome.SuppressedQuestions {\n"
    + T + T + "row.Suppressed = append(row.Suppressed, suppressed.MaterialKey+\" (источник: \"+suppressed.Source+\")\")\n"
    + T + "}\n"
    + T + "// The active alternatives of the outcome (Story 2.11): every word\n"
    + T + "// carries its honest limitation text — the panel renders the word\n"
    + T + "// with the text and the next steps of the outcome (AC-11: the\n"
    + T + "// variants describe the result, the limitations and the next step\n"
    + T + "// in plain words).\n"
    + T + "for _, word := range outcome.ActiveAlternatives {\n"
    + T + T + "row.Alternatives = append(row.Alternatives, policyAlternativeRow{\n"
    + T + T + T + "Word: word, WordText: accuracyQualifierText(word),\n"
    + T + T + T + "LimitationText: casesDomain.AlternativeLimitationText(word),\n"
    + T + T + "})\n"
    + T + "}\n"
    + T + "return row\n"
)

# 7. prepareOwnershipPanel: the transfer card words + the reply qualifier word ---
po_pending_old = (
    T + "if state.Pending != nil {\n"
    + T + T + "panel.HasPending = true\n"
    + T + T + "panel.PendingTarget = ownership.PendingTargetName\n"
    + T + T + "panel.PendingDeadline = formatTime(state.Pending.DeadlineAt)\n"
    + T + T + "panel.PendingRequestID = state.Pending.RequestID.String()\n"
)
po_pending_new = (
    T + "if state.Pending != nil {\n"
    + T + T + "panel.HasPending = true\n"
    + T + T + "panel.PendingTarget = ownership.PendingTargetName\n"
    + T + T + "panel.PendingDeadline = formatTime(state.Pending.DeadlineAt)\n"
    + T + T + "panel.PendingRequestID = state.Pending.RequestID.String()\n"
    + T + T + "// The Story 2.11 transfer card: the current preference words the\n"
    + T + T + "// receiving operator reads BEFORE the ack (AC-1 «передаются\n"
    + T + T + "// каждому последующему Response Owner»); an empty word set stays\n"
    + T + T + "// empty — the honest absence, never an invented default.\n"
    + T + T + "preferenceWords := []string{}\n"
    + T + T + "for _, pref := range view.Accommodations.Current {\n"
    + T + T + T + "if !pref.Cleared {\n"
    + T + T + T + T + "word := accommodationKeyText(pref.Key)\n"
    + T + T + T + T + "if pref.Value != \"\" {\n"
    + T + T + T + T + T + "word += \" = \" + pref.Value\n"
    + T + T + T + T + "}\n"
    + T + T + T + T + "preferenceWords = append(preferenceWords, word)\n"
    + T + T + T + "}\n"
    + T + T + "}\n"
    + T + T + "if len(preferenceWords) > 0 {\n"
    + T + T + T + "panel.AccommodationWords = strings.Join(preferenceWords, \"; \")\n"
    + T + T + "}\n"
)
po_reply_old = (
    T + "for _, card := range ownership.Replies {\n"
    + T + T + "row := replyRowView{\n"
    + T + T + T + "ReplyID:   card.Reply.ReplyID.String(),\n"
    + T + T + T + "Channel:   card.Reply.Channel,\n"
    + T + T + T + "Content:   card.Reply.Content,\n"
    + T + T + T + "ClassText: replyClassText(card.Reply.ReplyClass),\n"
    + T + T + T + "Epoch:     strconv.FormatInt(card.Reply.Epoch, 10),\n"
    + T + T + T + "CreatedAt: formatTime(card.Reply.CreatedAt),\n"
    + T + T + T + "State:     card.OutcomeKind,\n"
    + T + T + T + "StateText: replyStateText(card.OutcomeKind),\n"
    + T + T + T + "OwnerText: card.Reply.OwnerRef.String(),\n"
    + T + T + "}\n"
)
po_reply_new = (
    T + "for _, card := range ownership.Replies {\n"
    + T + T + "row := replyRowView{\n"
    + T + T + T + "ReplyID:   card.Reply.ReplyID.String(),\n"
    + T + T + T + "Channel:   card.Reply.Channel,\n"
    + T + T + T + "Content:   card.Reply.Content,\n"
    + T + T + T + "ClassText: replyClassText(card.Reply.ReplyClass),\n"
    + T + T + T + "Epoch:     strconv.FormatInt(card.Reply.Epoch, 10),\n"
    + T + T + T + "CreatedAt: formatTime(card.Reply.CreatedAt),\n"
    + T + T + T + "State:     card.OutcomeKind,\n"
    + T + T + T + "StateText: replyStateText(card.OutcomeKind),\n"
    + T + T + T + "OwnerText: card.Reply.OwnerRef.String(),\n"
    + T + T + "}\n"
    + T + T + "if card.Reply.AccuracyQualifier != \"\" {\n"
    + T + T + T + "row.QualifierText = accuracyQualifierText(card.Reply.AccuracyQualifier)\n"
    + T + T + "}\n"
)

patch("internal/transport/http/cases_view.go", [
    (cv_old, cv_new),
    (reply_old, reply_new),
    (own_old, own_new),
    (prep_old, prep_new),
    (pol_fields_old, pol_fields_new),
    (ppr_fields_old, ppr_fields_new),
    (ppr_loop_old, ppr_loop_new),
    (po_pending_old, po_pending_new),
    (po_reply_old, po_reply_new),
])

print("ALL PATCHES OK")
