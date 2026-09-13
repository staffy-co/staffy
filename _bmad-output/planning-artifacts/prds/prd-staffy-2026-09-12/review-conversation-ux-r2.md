---
title: 'Staffy PRD — Conversational UX Re-review Round 2'
status: complete
created: '2026-09-13'
reviewed:
  - prd.md
  - delivery-slices.md
  - addendum.md
  - review-conversation-ux.md
---

# Reviewer Gate Round 2 — Conversational UX and Omnichannel Quality

## Gate verdict

**PASS — CX-01–CX-06 resolved; no unresolved Critical/High findings.**

The edited PRD now defines the six previously missing interaction contracts as observable behavior and connects them to controlled-stage evidence in `delivery-slices.md`. The eight Medium edge paths CX-07–CX-14 are also covered without forcing provider/session implementation into the PRD. No new conversational-UX finding was found in the re-review.

## Severity counts

| Severity | Count |
|---|---:|
| Critical | 0 |
| High | 0 |
| Medium | 0 |
| Low | 0 |

## High blockers CX-01–CX-06

### CX-01 — resolved

FR-12 now represents multiple intents as separate Work Items with their own state, announces ambiguous topic switches, supports pause/resume and recaps only changed or unresolved elements before Commitment (`prd.md:403–412`). NFR-29 retains the corresponding multi-topic/repair acceptance requirement.

**Verification:** confirmed facts, media and confirmation state no longer need to leak between simultaneous requests for the requirement to pass.

### CX-02 — resolved

FR-25 defines voice Confirmation as an unambiguous response to one presented material Commitment and explicitly rejects silence, clipped/overlapped speech, low-confidence recognition and generic backchannels such as «угу», requiring focused repair or another modality (`prd.md:533–541`). NFR-31 additionally separates generated/provider-played/customer-confirmed output and requires semantic repair after interruption.

**Verification:** natural-sounding but ambiguous audio cannot satisfy a material Confirmation.

### CX-03 — resolved

FR-12 resumes from the last safe checkpoint and tells the customer what did and did not complete after disconnect/reconnect (`prd.md:403–412`). FR-17 says a disconnect during terms, Confirmation, Action or Handoff implies neither consent nor success and routes through reconnect, callback, protected continuation or human fallback (`prd.md:454–464`).

**Verification:** the journey has a customer-visible recovery contract, not merely a reconnect test case.

### CX-04 — resolved

FR-80 requires preview to include an end-to-end rehearsal of both customer and operator experience, a sandboxed/non-mutating Action, Handoff/return, expected-versus-actual output and the exact version being published (`prd.md:1070–1079`). The FR-78–FR-82 delivery slice makes this guided experience mandatory by the third-client/platform proof while preserving the underlying controlled-stage invariants (`delivery-slices.md:22`).

**Verification:** isolated voice/model previews are insufficient to pass readiness.

### CX-05 — resolved

FR-13 now requires a customer-facing message for both AI→human and human→AI ownership transitions, including the current responder, transferred context and next step (`prd.md:414–423`). FR-35 covers loss of availability after acknowledgement, and FR-83 plus its delivery slice require full Handoff/return/recovery rehearsal.

**Verification:** customer-visible identity and accountability survive both handoff directions.

### CX-06 — resolved

NFR-32 requires separate voice/text reports segmented at least by journey, channel/modality, Conversation Mode and critical transition; a route-blocking failure cannot be hidden by an aggregate (`prd.md:1344–1346`). FR-84 freezes the Coverage Matrix and gate definitions before evaluated results, while SM-C13 prevents success by Handoff substitution and SM-C14 blocks the affected route on critical UX/safety failure (`prd.md:1112–1121`, `prd.md:1393–1394`).

**Verification:** aggregate conversational quality can no longer promote a version that fails a smaller critical cohort.

## Medium edge paths CX-07–CX-14

### CX-07 — resolved

UJ-P1 now begins before credentials/setup and covers desired outcome, Capability fit, explicit gaps, human obligations, indicative packaging/usage, pilot evidence plan and proceed/assisted-extension/no-fit outcomes (`prd.md:59–65`). UJ-P2 remains the setup journey.

### CX-08 — resolved

FR-78 defines an unpublished draft with owner and `incomplete/blocked/ready` states, save/resume, delegation of missing input and approval summary; partial configuration must not appear active (`prd.md:1049–1058`).

### CX-09 — resolved

FR-17 gives a customer-facing state for a long but active tool call, prohibits fabricated ETA and offers waiting, callback or Human Handoff (`prd.md:454–464`). This is safer than promising cancellation of a potentially dispatched mutating operation.

### CX-10 — resolved

FR-16 requires the leading channel to show link delivery/waiting/completion/expiry, correlate one linked submission and support cancelling the wait while preserving an alternative route (`prd.md:443–452`).

### CX-11 — resolved

FR-16 gives feedback for unclear, damaged or ambiguous media, identifies its Work Item and presents extracted material facts for correction; FR-82 also requires usability, missing-angle/field and focused-correction feedback (`prd.md:443–452`, `prd.md:1090–1100`).

### CX-12 — resolved

FR-15 explicitly covers voicemail, shared/general phones, third-party answers and unverified identity: no sensitive reason/details are disclosed and none of these states count as Confirmation (`prd.md:433–441`).

### CX-13 — resolved

FR-36 adds preferred channel, time and language plus available accommodations to the decision-ready Handoff context (`prd.md:647–654`). Together with FR-12, the operator receives confirmed facts without forcing repetition.

### CX-14 — resolved

FR-17 detects unsupported language/code-switching and requires an honest configured transfer, callback or text fallback before high-stakes qualification (`prd.md:454–464`). Operating Profile/Gate Definition freezes supported languages and route coverage, while NFR-29/NFR-32 cover mishearing/repair and segmented acceptance.

## Cross-artifact consistency

- `delivery-slices.md` requires FR-9–FR-17 transition/degraded/reconnect tests, FR-18–FR-26 rejection of ambiguous Confirmation, FR-34–FR-41 full acknowledgement/reassignment/recovery behavior, FR-78–FR-82 preview and NFR-29–NFR-32 segmented conversation acceptance before live.
- `addendum.md` keeps media, providers, session/transport and operator-surface implementation replaceable while preserving typed events, interruption, heard-output, Handoff and testing patterns. It does not weaken any customer-observable contract above.
- No implementation-specific protocol was necessary to close CX-01–CX-14; the PRD remains behavioral and testable.

## Final gate statement

Unresolved Critical: **0**. Unresolved High: **0**. Conversational-UX gate is passed; all fourteen prior findings are resolved at PRD level and remain assigned to explicit controlled/live evidence gates.
