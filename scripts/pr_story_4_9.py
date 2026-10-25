#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 4-9 — python-JSON body assembly (precedent pr_story_4_8.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ["STAFFY_GH_TOKEN"]
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/artemiosu/staffy"

body = f"""## Story 4.9 — Truthful order status of ABCP, 1C and the supplier (first slice: the Order Status Projection + the Supplier Order Fact Receipt)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **The Order Status Projection = a PURE read-time derivation over the append-only material of the three machines** (the operation+receipt of 4-6, the verification events of 4-7, the correction events of 4-8) plus the supplier facts of this slice — ZERO writes (AC-2 structurally: the domain imports only platform/id, no repository exists there; the computed representation never becomes a new source of truth — the projection re-reads at every panel open).
- **The five lifecycle stages BY NAME** {{client_order_created, pending_manager_verification, manager_verified, supplier_formed, supplier_sent}}: «formed» and «sent» are DIFFERENT states; the send is the HONEST AGGREGATE — supplier_sent confirms ONLY when EVERY recorded formation is covered by a sent record of the SAME supplier order ID (a partial send names every formation and stays unconfirmed — the false send claim is impossible STRUCTURALLY, AC-15).
- **The Supplier Order Fact Receipt = the ONLY new write path**: the append-only human receipts {{formed, sent}} recorded by a manager with the supplier order ID and TWO times — the named happened_at (the manager's word, the provider-observed precedent of 2-7) and the server recorded_at (never mixed). NO supplier-send field/route/action exists anywhere (fence OQ-3: passing OQ-3/OQ-5 does not authorize supplier-send — the facts ONLY record the human reality).
- **The idempotency BY KEY (AC-9 without a webhook — no integrations exist, AG-4)**: the UNIQUE full fact key (tenant, case, operation, kind, supplier_order_id) — the repeated command answers the SAVED fact with replay=true, never a second row; the raced twin re-reads and replays honestly.
- **The record ladder**: the sent fact lands only AFTER the formed fact of the SAME supplier order ID (the discipline of the full history — the refusal word names it); the facts are NEVER guarded against a «check bypass» (the human reality records as it is — the panel shows BOTH facts, AC-12 spirit).
- **The source of every fact BY NAME (AC-5/AG-4)**: {{system_receipt, manager_record}} — the human receipt NEVER renders as a system one; the per-system blocks stay SEPARATE (AC-2: ABCP — the dispatch receipt OR the manual contour OR the reread of 4-8; 1C — unknown, OQ-4: the creation is never a synchronization; the supplier — the facts by name or «не записано»); the divergent external IDs show BOTH separately — the favorable version is never chosen (AC-8).
- **The ordered fold (AC-9)**: the shuffled material yields the SAME projection (the unit shuffle test by DeepEqual); the old event never rolls the new one back; the timeline orders by the named times with the stable anchor tie.
- **The freshness without polling (AC-10/AD-14)**: the last confirmed fact time + the live-derivation word + the no-schedule word (no workers, no timers — the stale value never renders as current); the three SEPARATE deadline words (AC-11: the estimate of the offer material / the confirmed date does not exist / the actual event — the sent record with its time).
- **The special state never hides behind «в работе» (AC-12)**: the open correction speaks first; the cancellation is the 4-10 boundary word; the dispute words (AC-14: the append-only history never rewrites — the new status version is the new events); NO notifications are built (OQ-1/OQ-2, the 2-13 Never precedent — the panel is the ONLY status surface; the delivery/read fields are zero).
- **The screen «Статус заказа» (AC-16)**: the stages as WORDS with the evidence pins (the time/the source BY NAME/the external ID/the owner) — never colors only; the per-system blocks; the transition feed; the three deadline words; the client retell words; the record form (the kind select + the supplier order ID + the named time); a11y at 320px.

### Steps
- step-01 (e94106f): migration 00008 (1 append-only table, RLS/CHECK mirrors, the UNIQUE full fact key) + the order_status domain (the dictionaries/the pure projection/the ordered fold/the guard) + unit (the shuffle by DeepEqual)
- step-02 (7bafeba): the port/store (the advisory lock, the fact_no ladder, the UNIQUE backstop) + the app service (2 record commands with the replay BY KEY + the honest anchor; the view assembly by the EXISTING port reads) + the api surface + unit
- step-03 (cc446c0): the «Статус заказа» panel + 2 JSON routes + the PRG form + OpenAPI (2 paths/3 closed-shape schemas/enum mirrors with the fence probes) + wire tests + the journeys/production wiring
- step-04 (a222e04): the integration eval-matrix of 10 rows (AC-15 by name, seed 771) + journeys z46 (the 51st) + the honest find-fix (the panel-form CSS min-width of the datetime-local)
- step-05 (e576dbd): QA self-review (verdict: NO Major findings) + closure

### Regression
build/vet OK · unit 30 packages · integration ×2 TZ=Europe/Moscow (59.3s/56.6s) · journeys 51/51 (z46 — the new 51st, 4.0m) · step-05 scan CLEAN (secrets/CJK/mixed/supplier-send/notification/price = 0; frozen machines 2-7/2-13/3-6/persist/terms 2-4/commands 2-15 AND the 4-6..4-8 panels untouched; SQL = exactly 1 migration 00008 append-only + the UNIQUE full fact key).

Epic 4: 9/11.
"""

payload = json.dumps({
    "title": "Story 4.9 — Truthful order status of ABCP, 1C and the supplier (Order Status Projection + Supplier Order Fact Receipt)",
    "head": "codex/story-4-9-truthful-order-status",
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
