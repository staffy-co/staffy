#!/usr/bin/env python3
# step-03 patch A: api/api.go (CaseView + Accommodations, WorkOutcome + 2.11 fields),
# api/ownership.go (SendClientReplyCommand + AccuracyQualifier), port.go (+AccuracyQualifier)
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

# --- api/api.go: CaseView field ---
api_caseview_old = (
    T + "// The Story 2.10 section: the continuation links of the case with\n"
    + T + "// the read-time state derivations, the delivered binding of the\n"
    + T + "// outgoing replies, the use accounting against the limit and the\n"
    + T + "// event feed (the FR-16 statuses — never stored words).\n"
    + T + "Continuations ContinuationView `json:\"continuations\"`\n"
    + "}"
)
api_caseview_new = (
    T + "// The Story 2.10 section: the continuation links of the case with\n"
    + T + "// the read-time state derivations, the delivered binding of the\n"
    + T + "// outgoing replies, the use accounting against the limit and the\n"
    + T + "// event feed (the FR-16 statuses — never stored words).\n"
    + T + "Continuations ContinuationView `json:\"continuations\"`\n"
    + T + "// The Story 2.11 section: the recorded communication preferences\n"
    + T + "// of the case — the CURRENT set (the read-time derivation), the\n"
    + T + "// append-only revision history, the key dictionary of the record\n"
    + T + "// form and the channel dictionary of the preferred_channel value\n"
    + T + "// (an OPERATIONAL setting, never a fact).\n"
    + T + "Accommodations AccommodationView `json:\"accommodations\"`\n"
    + "}"
)

# --- api/api.go: WorkOutcome 2.11 fields ---
api_outcome_old = (
    T + "LinkID              id.UUID   `json:\"link_id,omitempty\"`\n"
    + T + "ContinuationIssued  bool      `json:\"continuation_issued,omitempty\"`\n"
    + T + "ContinuationExpires time.Time `json:\"continuation_expires_at,omitempty\"`\n"
    + "}"
)
api_outcome_new = (
    T + "LinkID              id.UUID   `json:\"link_id,omitempty\"`\n"
    + T + "ContinuationIssued  bool      `json:\"continuation_issued,omitempty\"`\n"
    + T + "ContinuationExpires time.Time `json:\"continuation_expires_at,omitempty\"`\n"
    + T + "// The Story 2.11 material: the accommodation record of the command\n"
    + T + "// (the record id, the key word and the clearing mark — the revision\n"
    + T + "// is the server-side truth of the store, the stored outcome never\n"
    + T + "// carries it) and the accuracy qualifier word the sales reply\n"
    + T + "// passed through (the row of the reply carries it — the honest\n"
    + T + "// limitation word of the outcome's active set).\n"
    + T + "AccommodationRecordID id.UUID `json:\"accommodation_record_id,omitempty\"`\n"
    + T + "AccommodationKey      string  `json:\"accommodation_key,omitempty\"`\n"
    + T + "AccommodationCleared  bool    `json:\"accommodation_cleared,omitempty\"`\n"
    + T + "AccuracyQualifier     string  `json:\"accuracy_qualifier,omitempty\"`\n"
    + "}"
)

patch("internal/cases/api/api.go", [
    (api_caseview_old, api_caseview_new),
    (api_outcome_old, api_outcome_new),
])

# --- api/ownership.go: SendClientReplyCommand + AccuracyQualifier ---
own_cmd_old = (
    T + "LinkID         id.UUID\n"
    + T + "IdempotencyKey string\n"
    + T + "Now            time.Time\n"
    + "}"
)
own_cmd_new = (
    T + "LinkID         id.UUID\n"
    + T + "// The Story 2.11 accuracy qualifier: the SALES reply against an\n"
    + T + "// outcome with active alternatives MUST carry the qualifier word of\n"
    + T + "// the outcome's active set (the absence of media never becomes an\n"
    + T + "// invented exact result); without alternatives the word stays EMPTY\n"
    + T + "// (the field has exactly one meaning — the playbook_step mirror).\n"
    + T + "AccuracyQualifier string\n"
    + T + "IdempotencyKey    string\n"
    + T + "Now               time.Time\n"
    + "}"
)

own_comment_old = (
    "// answer — always acknowledge_concern); without an objection the empty\n"
    "// word keeps the old path byte-identical."
)
own_comment_new = (
    "// answer — always acknowledge_concern); without an objection the empty\n"
    "// word keeps the old path byte-identical. Story 2.11 extends the material\n"
    "// with the accuracy qualifier word: the SALES reply against an outcome\n"
    "// with active alternatives carries the honest limitation word of the\n"
    "// outcome's active set (a subset of the OUTCOME, never the dictionary in\n"
    "// general); without alternatives the word stays empty."
)

patch("internal/cases/api/ownership.go", [
    (own_cmd_old, own_cmd_new),
    (own_comment_old, own_comment_new),
])

# --- port.go: WorkCommandParams + AccuracyQualifier (the send_client_reply extension) ---
port_old = (
    T + "AccommodationRecordID id.UUID\n"
    + T + "AccommodationKey      string\n"
    + T + "AccommodationValue    string\n"
    + T + "AccommodationCleared  bool\n"
    + T + "AccommodationNote     string\n"
    + "}"
)
port_new = (
    T + "AccommodationRecordID id.UUID\n"
    + T + "AccommodationKey      string\n"
    + T + "AccommodationValue    string\n"
    + T + "AccommodationCleared  bool\n"
    + T + "AccommodationNote     string\n"
    + T + "// The accuracy qualifier word of the send_client_reply extension: the\n"
    + T + "// SALES reply against an outcome with active alternatives carries the\n"
    + T + "// honest limitation word of the outcome's active set (the word lands\n"
    + T + "// in the reply row; the store re-validates the gate under the locks).\n"
    + T + "AccuracyQualifier string\n"
    + "}"
)

patch("internal/cases/port/port.go", [(port_old, port_new)])

print("ALL PATCHES OK")
