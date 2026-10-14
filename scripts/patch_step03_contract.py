#!/usr/bin/env python3
# step-03 patch H: contracts/openapi/contract_test.go — the honest v2 mirror
# (the mechanical extension of the policy assertions) + the accommodation
# contract test
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

# The honest v2 mirror of the policy dictionaries (the 2.6 «mechanical
# extensions» precedent — the dictionaries of the domain decide, the
# contract mirrors).
reason_old = (
    T + "if reasonEnum, _ := mapping(t, outcomeProperties, \"reason\")[\"enum\"].([]any); len(reasonEnum) != 6 {\n"
    + T + T + "t.Errorf(\"reason must carry the six-word enum (mandatory_missing is the reserved pack word), got %v\", reasonEnum)\n"
    + T + "}\n"
    + T + "if ruleEnum, _ := mapping(t, outcomeProperties, \"rule_version\")[\"enum\"].([]any); len(ruleEnum) != 1 || ruleEnum[0] != \"intake-policy-v1\" {\n"
    + T + T + "t.Errorf(\"rule_version is the fixed intake-policy-v1 word, got %v\", ruleEnum)\n"
    + T + "}\n"
)
reason_new = (
    T + "if reasonEnum, _ := mapping(t, outcomeProperties, \"reason\")[\"enum\"].([]any); len(reasonEnum) != 7 {\n"
    + T + T + "t.Errorf(\"reason must carry the seven-word enum (no_alternative_path is the Story 2.11 honest stop), got %v\", reasonEnum)\n"
    + T + "}\n"
    + T + "if ruleEnum, _ := mapping(t, outcomeProperties, \"rule_version\")[\"enum\"].([]any); len(ruleEnum) != 2 || ruleEnum[0] != \"intake-policy-v1\" || ruleEnum[1] != \"intake-policy-v2\" {\n"
    + T + T + "t.Errorf(\"rule_version carries the v1/v2 words (the historical rows stay readable), got %v\", ruleEnum)\n"
    + T + "}\n"
)

# The snapshot: the preferences map is the v2 fourth input.
snapshot_required_old = (
    T + "snapshot := mapping(t, schemas, \"PolicyInputSnapshot\")\n"
    + T + "snapshotRequired, _ := snapshot[\"required\"].([]any)\n"
    + T + "for _, word := range []string{\"proposal_id\", \"schema_version\", \"mode_version\", \"owner_epoch\", \"facts_digest\", \"work_item_version\", \"rule_version\"} {\n"
)
snapshot_required_new = (
    T + "snapshot := mapping(t, schemas, \"PolicyInputSnapshot\")\n"
    + T + "snapshotRequired, _ := snapshot[\"required\"].([]any)\n"
    + T + "for _, word := range []string{\"proposal_id\", \"schema_version\", \"mode_version\", \"owner_epoch\", \"facts_digest\", \"work_item_version\", \"rule_version\", \"preferences\"} {\n"
)

patch("contracts/openapi/contract_test.go", [
    (reason_old, reason_new),
    (snapshot_required_old, snapshot_required_new),
])

print("ALL PATCHES OK")
