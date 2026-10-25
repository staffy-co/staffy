#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 4-10 — python-JSON body assembly (precedent pr_story_4_9.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ["STAFFY_GH_TOKEN"]
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/artemiosu/staffy"

body = f"""## Story 4.10 — Safe servicing of the EXISTING order (first slice: the Order Servicing Request + the Order Lookup BY KEY + the Identity Proof Gate + the CancelFeasibility)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **The Order Servicing Request = the ONLY new write path**: the append-only recorded event of the REPEAT contact (FR-62) — the operator with the review scope records WHAT THE CLIENT ASKED: the intent word (the 8-word dictionary), the TYPED identifier the client named and the LIVE proof pins of the contact case assurance copied at the moment of the record (the scope, the level and the time of the check are preserved — AC-4). The request fixes the ask, it NEVER claims the result (AC-6); the resolved anchor references the EXISTING operation — a new one is NEVER created (FR-62 structurally: the count probe of the matrix).
- **The Identity Proof Gate reads ONLY the verification records (FR-11/2-3 structural)**: the disclosure — INCLUDING the very existence of the order — opens ONLY at the LIVE verified level of the contact case (the new AssuranceSource port over the cases API, the CaseMaterialSource composition-root precedent; a nil port = the honest refusal, NEVER a masked «ok» pin). The Caller ID, the chat profile name and the knowledge of the order number are NEVER a proof. The denied client words carry the gate word with the LIVE allowed methods + the fallback words of 2-3 — WITHOUT the lookup outcome (the AC-1 leak probe pins it by name in the unit, integration AND the rendered page).
- **The Order Lookup BY KEY — never an enumeration**: the exact match over the RECORDED material of the contour only (the committed receipts of 4-6 UNION the rereads of 4-8 for the ABCP kind; the human facts of 4-9 for the supplier kind — AG-4/OQ-18: no live external read exists); the outcomes {{unique_match, not_found, ambiguous}} BY NAME — not_found answers the GENERIC word (never a disclosure of another client's orders, AC-2/AC-12), ambiguous answers the word WITHOUT the list; the SAME operation matched by several sources stays the unique match (the sources ride BY NAME).
- **The idempotency BY KEY + the trust ladder**: the UNIQUE full request key (tenant, case, actor, intent, identifier_kind, identifier_value, proof_state) — the repeated command at the SAME trust state answers the SAVED request with replay=true (never a second row); the CHANGED trust state (none → verified) is a NEW row — the ladder stays visible in the history (both rows live, the SQL truth pins it).
- **The CancelFeasibility = the FOUR read-time states (AC-7)**: the recorded request (append-only) / the prepared action does not exist (cancel is absent from the Action Catalog — the OQ-3 fence; the 4-1 declaration names a command that does not exist — deferred-work №215) / the confirmed cancellation does not exist (the receipt is absent — the cancel is NEVER promised) / the fourth state: the supplier facts {{formed, sent}} of 4-9 name the touched order — «затронул поставщика — передаётся человеку» (AG-4: the manual contour; the order is NOT cancelled until the human confirmation). NO cancel execution, NO change execution exists anywhere (OQ-3/AG-4); the claim/dispute/human intents route to the EXISTING contours 3-8/2-13 BY WORDS (the legal liability is never admitted, the compensation is never promised; the recovery/handoff machines live in the manager's hands — the autonomous-claim probe pins the zero).
- **The phone/email lookup kinds do NOT exist (OQ-16)**: the cross-case profile is never built — the guard refuses the out-of-dictionary kinds with the boundary word; the client delivery does NOT exist (OQ-1/OQ-2, the 4-9 Never precedent — the panel is the ONLY surface, the operator retells the client words by hand).
- **The screen «Обслуживание заказа» (AC-16)**: the request feed with the proof pins and the disclosure state as TEXT (never colors only), the LIVE client words (the allowed disclosure re-derives over the live 4-9 projection + the intent boundary word), the four cancel states, the stable word-links «Позвать человека» (the 2-13 handoff section) and «Вернуться в разговор» (the conversation mode surface); a11y at 320px.

### Steps
- step-01 (314997e): migration 00009 (1 append-only table, RLS/CHECK mirrors, the UNIQUE full request key at the same trust state) + the order_servicing domain (the 8/2/3/4/2 dictionaries + DeriveLookupOutcome + DeriveCancelFeasibility + DeriveServicingClientWords + GuardOrderServicingRequest + the 2-3 mirror test) + unit
- step-02 (444a662…3d8ea3b): the ports (OrderServicingRepository + the NEW AssuranceSource) + the store (the lookup over the receipts ∪ the rereads ∪ the facts) + the app (RecordOrderServicingRequest with the LIVE proof pins + the view with the LIVE derivations) + the api surface + unit
- step-03 (3cbb576): the «Обслуживание заказа» panel + the JSON route + the PRG form + the assurancesource adapter + OpenAPI (1 path/2 closed-shape schemas/5 enum mirrors with the fence probes) + wire tests
- step-04 (1d8b426): the integration eval-matrix of 10 rows (AC-14 by name, seed 772) + journeys z47 (the 52nd) + the fixture + the honest find-fixes (the pack-session prefix, the disclosure cell, the zero proof times, the PRG flag on the redirect URL, the text-channel verified method)
- step-05 (d4f6218): QA self-review (verdict: NO Major findings) + closure

### Regression
build/vet OK · unit all packages · integration ×2 TZ=Europe/Moscow (65.2s/63.8s) · journeys 52/52 (z47 — the new 52nd, 4.3m) · step-05 scan CLEAN (secrets/CJK/mixed/supplier-send/cancel-execution/notification/price = 0; frozen machines 2-3/2-13/3-6/3-8/4-9 AND the 4-6..4-9 panels untouched; SQL = exactly 1 migration 00009 append-only + the UNIQUE full request key + the CHECK mirrors).

Epic 4: 10/11.
"""

payload = json.dumps({
    "title": "Story 4.10 — Safe servicing of the EXISTING order (Order Servicing Request + Lookup BY KEY + Identity Proof Gate + CancelFeasibility)",
    "head": "codex/story-4-10-safe-existing-order-servicing",
    "base": "main",
    "body": body,
}).encode()

request = urllib.request.Request(
    f"{API}/pulls",
    data=payload,
    method="POST",
    headers={
        "Authorization": f"Bearer {TOKEN}",
        "Accept": "application/vnd.github+json",
        "Content-Type": "application/json",
    },
)
try:
    with urllib.request.urlopen(request) as response:
        result = json.load(response)
        print(json.dumps({"number": result["number"], "url": result["html_url"], "head": result["head"]["sha"]}, ensure_ascii=False))
except urllib.error.HTTPError as error:
    print(f"HTTP {error.code}: {error.read().decode()[:500]}")
    sys.exit(1)
