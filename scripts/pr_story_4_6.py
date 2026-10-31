#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""PR story 4-6 — python-JSON body assembly (lesson 4-4: sha-pin of the head
in the body; bash artifacts are excluded)."""
import json
import os
import sys
import urllib.request

TOKEN = os.environ["STAFFY_GH_TOKEN"]
HEAD_SHA = sys.argv[1]
API = "https://api.github.com/repos/staffy-co/staffy"

body = f"""## Story 4.6 — Creating a single client order in ABCP (first slice: Order Commitment)

Merge `{HEAD_SHA}` (sha-pinned head of the branch, verified with a live GET before the merge).

### Contract (briefly)
- **Order = action through the Action Gateway**: the builder derives the order material ALIVE from the recorded selection of 4-5 (the pins of ALL FIVE inputs, positions, the known sum, the MANDATORY manager-check word AC-1) — the operator never inputs the composition.
- **The FIRST product entry of the action catalog**: `create_abcp_customer_order@1.0.0` (admission against the live declaration 4-1 `abcp-parts@1.1.0/command.prepare-client-order`, Idempotent=true → provider_idempotent).
- **Execute-time revalidation (AC-5)**: a NEW `OrderVersionSource` port in the 3-6 dispatcher ladder — the live re-derivation of the ONE canonical OrderDigest BEFORE the lease: any change of price/availability/composition → `dispatch_blocked` with the exact word; nil port → fail-closed.
- **Receipt with derived read-time words**: `pending_manager_verification` + client words + honest 1C-unknown + manager assignment (the stored 3-6 receipt stays three-worded).
- **The production barrier AG-4**: nil adapter → honest blocking + recommendation `order_creation` (migration 00025, reason 6→7) through the existing 2-13 machine.
- **NO sending to the supplier** (fence OQ-3: supplier_form/send is absent from the Action Catalog — not added by this slice).

### Steps
- step-01 (b265aa8): definition + order domain (guard by name, canonical digest, honest words)
- step-02 (0956e63): builder + OrderVersionSource + FENCE 4.5 ladders + receipt render + 00025
- step-03 (76fa7ed): «Customer Order» panel + builder routes + OpenAPI mirrors + wire tests
- step-04 (5f70283): integration eval-matrix of 10 rows (AC-15 by name) + journeys z43
- step-05 (204d84d): QA self-review (verdict: NO Major findings) + closure

### Regression
build/vet OK · unit 33 packages · integration ×2 TZ · journeys 48/48 · scan CLEAN (secrets/CJK/mixed/supplier-send/marketing = 0).

Epic 4: 6/11.
"""

payload = json.dumps({
    "title": "Story 4.6 — Creating a single client order in ABCP (Order Commitment)",
    "head": "codex/story-4-6-abcp-client-order",
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
