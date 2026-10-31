#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 4-7 — python-JSON body assembly (precedent pr_story_4_6.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ["STAFFY_GH_TOKEN"]
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 4.7 — Decision-ready order verification by the manager (first slice: Delegated Task over the committed receipt of 4-6)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **The verification task = an append-only domain in the action module** (the anchor is the committed operation/receipt of `create_abcp_customer_order` — NOT turn_ref as in 2-13): the lifecycle events {{assigned, acknowledged, approved, blocked}} + read-time words overdue/inapplicability; assignment ≠ acceptance (three truths by words: «назначен ≠ доставлено ≠ принял»).
- **The decision = TWO DIFFERENT actions (AC-16)**: approved = the Human Verification Receipt (an append-only event with the material pins: order_digest 64-hex, receipt identity, external ABCP Order ID, Work Items/evidence anchors) OR blocked = affected positions (checked against the operation composition) + a dictionary reason of 5 words + the correction launch word «контур 4-8».
- **The decision intake (AC-9/AC-14)**: the LIVE re-derivation of DeriveOrderCommitmentMaterial over the fresh CaseView by the SAME 3-6 ladder code — live digest ≠ the operation pin → the refusal BY NAME without the event; the read-time inapplicability of the former decision after the material change.
- **The CAS assignment epoch (AC-7)**: two managers deciding concurrently → exactly one wins, the loser gets the honest raced word with the CURRENT state; the stale epoch → «задача переназначена».
- **The expired deadline (AC-8)**: the read-time overdue word + the backup owner by name + the explicit reassignment fallback — NO automatic approval (structurally: the auto-approval event kind does not exist), NO workers/timers (AG-4/AD-14).
- **The screen «Проверка заказа»**: the decision-ready context by LIVE panel reads only (Vehicle+VIN 4-2, Part Request 4-3, positions+fitment anchors 4-4, Supplier Offer 4-5; the purchasing price ONLY by the Commercial scope of the actor — the live grant matrix), the technical line (Operation ID/idempotency/correlation/receipt/external ID/1C-unknown) + the MANDATORY manual-contour word + the fence honesty word (AC-13: the bypass by hands in ABCP is possible — Staffy guarantees its own contour only).
- **NO supplier-send** (fence OQ-3: no routes/fields/actions — the structural probe is zero), **NO external notification delivery** (2-13 Never: the honest words of the surface), **NO ownership transfer** (the 2-4 terms stay untouched).

### Steps
- step-01 (18fd6ac): migration 00006 (2 append-only tables, RLS/CHECK mirrors) + the order_verification domain (dictionaries/guards/read-time derivations/honest words) + unit
- step-02 (200d745): the app service of 5 commands (assign/acknowledge/approve/block/reassign) with the decision intake by the live re-derivation + the CAS store + unit (races via raceHook)
- step-03 (b5ba76b): the «Проверка заказа» panel + 5 JSON+PRG routes + OpenAPI mirrors + wire tests
- step-04 (0b60529): the integration eval-matrix of 10 rows (AC-15 by name) + journeys z44 (the 49th)
- step-05 (e214718): QA self-review (verdict: NO Major findings) + closure

### Regression
build/vet OK · unit 33 packages · integration ×2 TZ=Europe/Moscow · journeys 49/49 · step-05 scan CLEAN (secrets/CJK/mixed/supplier-send/marketing = 0; frozen machines untouched; SQL = exactly 1 migration 00006).

Epic 4: 7/11.
"""

payload = json.dumps({
    "title": "Story 4.7 — Decision-ready order verification by the manager (Delegated Task)",
    "head": "codex/story-4-7-manager-order-verification",
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
