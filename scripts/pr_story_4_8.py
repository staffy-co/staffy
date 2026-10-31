#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 4-8 — python-JSON body assembly (precedent pr_story_4_7.py:
sha-pin of the head in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ["STAFFY_GH_TOKEN"]
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 4.8 — Order correction before purchase (first slice: the Correction Work Item over the LAST CLOSED blocked decision of 4-7)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **The correction = an append-only domain in the action module** (the anchor is the committed operation of `create_abcp_customer_order` + the LAST CLOSED blocked verification decision — NOT turn_ref 2-13, NOT the operator's case-fact correction 2-15: the contours differ BY NAME): the lifecycle events {{opened, reassigned, reread, ceased}} + the CAS assignment epoch; the blocked positions and the reason word carry BY NAME inside the ONE creation transaction (the SQL-truth carry — the original blocked event is READ, never rewritten); the one-closure partial UNIQUE — a correction never closes twice.
- **The ladder of the anchor**: every correction pins the verification_no of its blocked decision — a closed correction CONSUMED its decision; the NEW correction opens only by a NEWER blocked decision (the ladder of the repeated correction, the history never rewrites).
- **TWO structural progression gates (AC-2/AC-9)**: an OPEN correction refuses the new verification assignment with the honest 409 «проверка вернётся после исправления» AND refuses the authorization of a NEW `create_abcp_customer_order` BEFORE the reservation transaction — NOTHING is consumed (the single_use hole of the spec finding is closed STRUCTURALLY; the replay idempotency stays BEFORE the gate, the 3-6 ladder and the 3-4 superseded mechanics stay untouched).
- **The replacement = a NEW proposal version by the existing EDIT ladder** (a replacement entity does NOT exist): the recompose lands the preview version+1 of the SAME thread; the customer confirms the new version — the old Confirmation becomes superseded (AC-15: the reread over the old triple refuses BY NAME).
- **The corrected closure = the reread record (AG-4: the manager's words — Staffy never reads ABCP)**: the pinned NEW confirmed version triple {{proposal, version, confirmation}} + the LIVE composition fence by the SAME re-derivation code as the ladder 4.5 and the 4.7 fence (a price/availability change AFTER the confirmation refuses with the EXPLICIT differences — the hidden degradation is impossible STRUCTURALLY, AC-13); the order returns to pending_manager_verification by a NEW verification task (the ladder verification_no, the former check never reuses, AC-12).
- **The ceased closure (AC-11)**: the dictionary reason of 3 words; client_refused carries the cancellation recommendation word «отмена заказа — ручной контур менеджера (контур 4-10)»; the absence of consent NEVER becomes a replacement.
- **The ABCP change = the manager's MANUAL contour ONLY (AG-4)**: no change command in the catalog (the in-code probe), no adapter, no route — the manual-contour word is the interface truth; NO supplier-send (fence OQ-3: the structural probe is zero); the recommended explanation for the customer: the explicit former → current differences ПО ПОЗИЦИИ (the canonical renders of 4-6) + the reason text + «прежде вариант поставщику не отправлялся» + the new-confirmation word + the honest channel word (OQ-1/OQ-2).
- **The screen «Коррекция заказа»**: THREE DIFFERENT forms (AC-16: «Записать перечит» / «Прекратить коррекцию» / «Переназначить») + the surgical pending-correction words on the «Заказ клиента» and «Проверка заказа» panels (the gates hold while the correction lives — honest words on every degradation path).

### Steps
- step-01 (7c887a9): migration 00007 (2 append-only tables, RLS/CHECK mirrors, the one-closure partial UNIQUE) + the order_correction domain (dictionaries/guards/read-time derivations/the per-position diff/20 honest words) + unit
- step-02 (5bb1b75): the app service of 4 commands (open with the SQL-truth carry/reassign/reread with the version validations and the composition fence/cease) + the CAS store + BOTH structural gates + unit (races via raceHook, the gates by live counters)
- step-03 (e329950): the «Коррекция заказа» panel + 4 JSON+PRG routes + the surgical panel words + OpenAPI mirrors + wire tests + the journeys/production wiring
- step-04 (cafb677): the integration eval-matrix of 10 rows (AC-15 by name, seed 770) + journeys z45 (the 50th) + the honest finds-fixes (the anchor ladder of the consumed blocked decisions; the active-409 order BEFORE the ladder)
- step-05 (8561d4e): QA self-review (verdict: NO Major findings) + closure

### Regression
build/vet OK · unit 33 packages · integration ×2 TZ=Europe/Moscow (55.2s/51.5s) · journeys 50/50 (z45 — the new 50th, 2.9m) · step-05 scan CLEAN (secrets/CJK/mixed/supplier-send/change = 0; frozen machines 2-13/3-6/persist/terms 2-4/commands 2-15 untouched; SQL = exactly 1 migration 00007 append-only + one-closure).

Epic 4: 8/11.
"""

payload = json.dumps({
    "title": "Story 4.8 — Order correction before purchase (Correction Work Item over the blocked decision)",
    "head": "codex/story-4-8-order-correction-before-purchase",
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
