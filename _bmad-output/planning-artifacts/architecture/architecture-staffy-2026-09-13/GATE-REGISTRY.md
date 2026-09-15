---
title: Staffy Architecture Gate Registry
status: adopted
registry_version: 1
instance_schema: gate-instance.schema.json
definition_registry: gate-definitions.json
instance_registry: gate-instances.json
updated: 2026-09-15
allowed_states: [open, in_test, pass, fail, expired, not_applicable]
source: PRD section 10 plus adopted Architecture decisions
---

# Staffy Architecture Gate Registry

## Executable contract

A **definition** says what must be proven. A scoped **instance** carries permission. `gate-definitions.json` and `gate-instances.json` are the authoritative machine registries, validated by their adjacent schemas; the tables below are human projections. Scope dimensions are present only when exact—there are no nulls or wildcards. Only exact state `pass` permits work, and only inside its scope. `not_applicable` requires a dated, approved rationale and grants no broader permission. All applicable gate instances are conjunctive: one missing, failed or expired gate keeps the route fail-closed. Order of work never overrides privacy, identity, safety or legal gates.

Transitions are append-only: `open -> in_test -> pass|fail`; any active state may become `expired`, and an approved scoped determination may become `not_applicable`. A failed, expired or superseded PASS is never overwritten. Reopening creates a new instance linked by `supersedes_instance_id`. Only the accountable owner(s) in the definition may approve. Distinct principals are required when more than one role is named unless that machine definition explicitly sets `allow_role_coincidence: true`; every required role must still appear in `approved_by`. Release admission compiles the Release Manifest's processor graph and required capability/action graph into instance dependencies, and fails if any edge lacks an exact applicable PASS.

## Traffic stages

- `synthetic`: no real customer or production PII; fixtures only.
- `closed_account_test`: approved test accounts with synthetic or explicitly minimized test data; no customer-facing promise.
- `controlled_real_customer`: real customers and PII in a bounded cohort with active human fallback; this is production data processing even when called a pilot.
- `live`: routine admitted customer use inside the approved Operating Profile.
- `scale`: expanded cohort, autonomy class, tenant count or load envelope.

Every definition's deadline refers to these stages. OQ-11 precedes any `controlled_real_customer` processing on its data-flow edge. OQ-13B precedes any `controlled_real_customer` voice route. AG-1 and affected AG-2 precede all controlled traffic.

Before OQ-11 PASS, integration tests use synthetic/minimized data and approved test accounts. Disabling recording does not authorize STT, model, text, tool, support-log or backup processing by an unapproved processor. Withdrawal advances a purpose-scoped control generation, stops affected capture/queued transfer, and preserves only legally required records.

## Gate definitions

| Definition | Criticality | Blocks / deadline | Accountable owner | Evidence required | Safe state until PASS |
|---|---|---|---|---|---|
| OQ-1 Oleg telephony | high | Connector completion and controlled voice | Oleg + Technical Lead | Account/contract inventory and recorded SIP/trunk, codec, identity, event, transfer, fallback and failure tests | `awaiting_verification/limited`; no unsupported transfer/caller claim |
| OQ-2 Jivo semantics | high | Publishing Oleg Jivo Connector | Oleg + Integration Lead | Account tests for IDs, reply, assignment, acknowledgement, writer exclusion, receipts, timeout and explicit return | Production shared inbox is human-only; Staffy-only requires isolated/enforced writer exclusion and tested takeover |
| OQ-3 ABCP prepared order | critical | First mutating ABCP controlled test | Oleg + ABCP integrator | Actual product/version, API rights, prepared-status mapping, notification, reread and enforceable human supplier-send fence | Neutral `pending_manager_verification`; manual reread and human receipt |
| OQ-4 ABCP -> custom 1C | critical | End-to-end Order test | Oleg + 1C integrator | State/ID mapping, correlation, version/time, duplicate behavior, reread/reconcile and terminal evidence | No terminal success; mismatch/unknown is manual reconciliation |
| OQ-5 Oleg fitment | critical | Publishing affected Autonomy Class | Oleg + fitment expert + Integration Lead | Recorded workflow; ABCP/Laximo tariff/account inventory; API/support confirmation; representative VIN -> vehicle -> diagram/node -> OEM -> cross/offer tests; allowed item classes | No inferred fitment/UI scraping; human task or licensed reusable catalog Connector |
| OQ-6 Oleg commercial pilot | medium | Signature/start of paid pilot | Product Owner + Oleg | Price, duration, budget, responsibilities, data/legal, success and exit terms | Indicative proposal only |
| OQ-7A Montagio cohort plan | high | Controlled work | Product Owner + Montagio Operations | Pre-registered UJ-M1/UJ-M2 allocation, denominator, eligibility, exclusions/crossover, matching and measurement owners | Simulation only; neither route substitutes for the other |
| OQ-7B Montagio route thresholds | high | Live/scale promotion | Product Owner + Montagio Operations | Baseline-derived per-route sample sufficiency and admission thresholds with approvers | Controlled learning only |
| OQ-8 Montagio fallback ownership | critical | Controlled work | Montagio Operations | Named supply, safety/technical, commercial/legal and recovery owners; acknowledgement windows, backup/escalation and drill | No immediate-help promise; named owner/next update or honest intake/callback |
| OQ-9 Montagio price | critical | Publishing each service price | Product Owner + Pricing/Operations | Market cut, approved rule, cost floor, source/version, freshness, exceptions and examples | Approved range/measurement/human route; no fixed-price promise |
| OQ-10 Montagio contractor/legal | critical | Public contractor offer and first public paid Order | Product Owner + Legal/Finance/Operations | Written contract party, payment, tax/KKT/receipt, insurance, warranty, claim/refund and subcontractor model; tabletop approval | `public_contractor_order=intake_only/no_promises`; closed test truthfully names actual parties |
| OQ-11 data route | critical | Production PII/recording for each provider route | Security/Privacy + Technical Lead | For each provider × route × artifact class × purpose: Russian perimeter, artifact/log/support/backup map, no-save behavior, legal basis/consent, retention/deletion, incident terms and fallback test | Synthetic/non-PII only or an independently PASSed route that does not call the unapproved processor |
| OQ-12 live voice baseline | high | Controlled-real-customer voice and later | Voice/Technical Lead | Route-specific p50/p95 response, playback stop, tool/action wait, reconnect, quality and cost for actual providers | No numeric immediacy promise; progress and verified fallback |
| OQ-13A availability policy | medium | Pilot commercial wording | Product Owner + Tenant | Explicit best-effort/no-SLA decision and customer wording | No contractual uptime claim |
| OQ-13B recoverability | critical | Controlled-real-customer voice and later | Technical Lead + Operations | Dated PostgreSQL/S3 restore, secret rebind, core startup, operation reconciliation and Media Node replacement drill; measured RTO/RPO | Synthetic or closed-account test only; objectives are not claimed achieved |
| OQ-14 quantitative admission | critical | Next autonomy/live/scale stage | Product Owner + Operations/QA | Versioned Gate Definition for correctness, critical errors, load, supply, re-quote, trust, conversation/recovery quality, cost and sample sufficiency | Manual scoped promotion; zero-tolerance counters still block |
| OQ-15 Staffy pricing/WTP | medium | Repeatable commercial offer | Product Owner | Units/limits/overage/full cost plus independent paid pilot, continued use/expansion and structured loss reasons | Pilot-specific pricing; no validated-retention claim |
| OQ-16 cross-case profile | high | Any cross-case personalization | Product Owner + Privacy/UX | Purpose/legal basis, identity match, visibility/correction/deletion, objection/consent, retention and eval | Case-scoped memory only |
| OQ-17 Staffy operator surface | high | Third Tenant without conforming Inbox | Architecture + Product Owner | Minimal Inbox passes FR-83 Human Surface Contract and accessibility evidence | Verified external Inbox required or Tenant not production-ready |
| OQ-18 post-pilot Connector | high | Roadmap/marketing and release of each new provider | Product Owner + Partnerships/Architecture | Demand, official API/terms, account capability, residency/security, cost, failure semantics and reusable contract fit | Not shown available; assisted extension only |
| OQ-19 State/Authority Matrix | critical | Low-level design and first mutating controlled test | Architecture + Owning-System owners | Per material field/entity: authority, external ID/version/time, projection, conflict rule, reconcile query, terminal evidence and contract tests | Field is `unknown`; no mutation or terminal success |
| AG-1 Pilot Load Envelope | critical | Any controlled traffic | Technical Lead + Operations | Per-Tenant/total voice+text concurrency, burst/duration, codec/bitrate, tool fan-out/timeouts, queue/handoff limits, provider limits, storage/retention and cost cap; test at approved safety factor with over-limit behavior | Simulation/load test only; no controlled traffic |
| AG-2 Accessibility admission | critical | Each affected release route | Product Owner + UX/QA | Automated plus manual keyboard, screen-reader, 320px/400% reflow, text spacing, forced colors, touch, reduced motion and equivalent voice/text/DTMF/human-path evidence | Affected surface/route does not publish |
| AG-3 Platform reuse proof | high | Claiming repeatable universality/scale | Product Owner + Architecture | Qualifying third external Tenant, real inputs/admin, supported Pack/Connectors, timer boundary, participant log, code-diff proof and controlled exercise receipt within one working day after valid inputs | Universality remains a hypothesis |
| AG-4 Runtime dependency readiness | high | Credential-bearing implementation | Technical Lead + Security | Concrete SecretStore, Russia-S3 and malware/safe-render choices; Go/PostgreSQL/pgx/sqlc/goose/River compatibility; license review, SBOM, vulnerability, migration and rollback tests | Local synthetic implementation only; no real credentials |

Mutation Capability Evidence must classify an operation as `provider_idempotent`, `query_reconcilable`, `compensatable` or `unsafe`. If neither idempotency nor authoritative reconciliation exists, retry is forbidden and the action is `assisted/human_only`, or one-shot plus mandatory human reconciliation only where explicitly approved.

## Current scoped instances

`—` means no evidence or permission exists yet; it is not a wildcard.

| instance_id | definition_id | scope | state | evidence_links | tested_at | expires_at_or_trigger | approved_by | permits |
|---|---|---|---|---|---|---|---|---|
| oq-1-oleg-voice | OQ-1 | Oleg / actual SIP account / voice | open | — | — | Provider/account/config change | — | — |
| oq-2-oleg-jivo | OQ-2 | Oleg / actual Jivo account / text | open | — | — | Tariff/account/API change | — | — |
| oq-3-oleg-abcp | OQ-3 | Oleg / ABCP prepared client order | open | — | — | ABCP account/version/status change | — | — |
| oq-4-oleg-abcp-1c | OQ-4 | Oleg / ABCP -> custom 1C Order | open | — | — | Integration/config/version change | — | — |
| oq-5-oleg-fitment | OQ-5 | Oleg / applicable Autoparts classes | open | — | — | Catalog/tariff/supplier mapping change | — | — |
| oq-6-oleg-paid-pilot | OQ-6 | Oleg commercial pilot | open | — | — | Contract change | — | — |
| oq-7a-montagio | OQ-7A | Montagio UJ-M1/UJ-M2 controlled cohort | open | — | — | Operating Profile/cohort change | — | — |
| oq-7b-montagio | OQ-7B | Montagio live/scale routes | open | — | — | Baseline/profile/route change | — | — |
| oq-8-montagio | OQ-8 | Montagio controlled operations | open | — | — | Owner/SLA/route change | — | — |
| oq-9-montagio-first-service | OQ-9 | Montagio first published service | open | — | — | Price source/market/freshness expiry | — | — |
| oq-10-montagio-public | OQ-10 | Montagio public contractor Order | open | — | — | Legal/commercial model change | — | — |
| oq-11-yandex-text | OQ-11 | Yandex / text AI / prompts+responses / service purpose | open | — | — | Terms/route/artifact/purpose change | — | — |
| oq-11-yandex-voice | OQ-11 | Yandex / realtime voice / audio+transcript+logs / service purpose | open | — | — | Terms/route/artifact/purpose change | — | — |
| oq-11-novofon | OQ-11 | Novofon / carrier+SIP route / audio+call metadata+provider logs / service purpose | open | — | — | Terms/route/artifact/purpose change | — | — |
| oq-11-oleg-sip | OQ-11 | Oleg actual SIP provider / audio+call metadata+provider logs / service purpose | open | — | — | Provider/account/terms/artifact change | — | — |
| oq-11-jivo | OQ-11 | Jivo / text+attachments+provider logs / service purpose | open | — | — | Terms/account/artifact/purpose change | — | — |
| oq-11-abcp-1c | OQ-11 | ABCP+custom 1C / identifiers+order payload+logs / order purpose | open | — | — | Terms/account/artifact/purpose change | — | — |
| oq-11-montagio | OQ-11 | Montagio web+application / customer facts+media+Order data+logs / service fulfillment purpose | open | — | — | Application/route/artifact/purpose change | — | — |
| oq-11-staffy-data | OQ-11 | Staffy PostgreSQL+Russia-S3+backups+telemetry+support / all retained artifact classes / declared purposes | open | — | — | Infrastructure/retention/support-path change | — | — |
| oq-12-novofon-yandex | OQ-12 | Montagio / Novofon+Yandex / voice | open | — | — | Provider/model/media build change | — | — |
| oq-12-oleg-voice | OQ-12 | Oleg / actual SIP+Yandex / voice | open | — | — | Provider/model/media build change | — | — |
| oq-13a-montagio | OQ-13A | Montagio pilot | pass | `evidence/oq-13a-montagio-best-effort.md` | 2026-09-15 | Commercial SLA or customer wording change | Founder as Product Owner + Montagio Tenant Owner | Best-effort pilot wording; no availability SLA |
| oq-13a-oleg | OQ-13A | Oleg pilot | open | — | — | Commercial terms or customer wording change | — | — |
| oq-13b-pilots | OQ-13B | Pilot live environments | open | — | — | Infrastructure/storage/runbook change | — | — |
| oq-14-first-promotion | OQ-14 | First post-baseline autonomy/live promotion | open | — | — | Release/profile/threshold change | — | — |
| oq-15-staffy-offer | OQ-15 | Repeatable Staffy commercial offer | open | — | — | Packaging/cost/market evidence change | — | — |
| oq-16-cross-case | OQ-16 | Cross-case Customer Profile | open | — | — | Purpose/legal/design change | — | — |
| oq-17-third-tenant | OQ-17 | First Tenant without conforming Inbox | open | — | — | Surface/UX/accessibility change | — | — |
| oq-18-post-pilot | OQ-18 | Each post-pilot channel/provider | open | — | — | Demand/API/terms/provider change | — | — |
| oq-19-oleg | OQ-19 | Oleg ABCP/custom-1C material fields | open | — | — | Field/API/status/account change | — | — |
| oq-19-montagio | OQ-19 | Montagio material fields/actions | open | — | — | Domain/API/legal model change | — | — |
| ag-1-first-controlled | AG-1 | First controlled voice+text pilot load | open | — | — | Load/provider/topology/profile change | — | — |
| ag-2-first-release | AG-2 | First controlled customer/operator surfaces | open | — | — | Affected UX/runtime change | — | — |
| ag-3-third-tenant | AG-3 | Third external Tenant proof | open | — | — | Pack/Connector/core change | — | — |
| ag-4-runtime | AG-4 | Credential-bearing V1 runtime | open | — | — | Dependency/provider/security change | — | — |

## Execution order

1. Before real credentials: AG-4. During Connector design: OQ-1–OQ-5 and initial OQ-19 matrices, using `synthetic` or `closed_account_test` data until applicable OQ-11 instances pass.
2. Before `controlled_real_customer`: every applicable OQ-11 data-flow instance, AG-1 and affected AG-2; for Montagio also OQ-7A, OQ-8 and OQ-9 if price is shown. Closed testing of OQ-10 must state actual contract/payment parties.
3. Before `controlled_real_customer` voice: OQ-12 and OQ-13B, plus every other applicable gate.
4. After baseline, before autonomy/live/scale promotion: OQ-7B and OQ-14.
5. Before paid/public commitments: OQ-6 for Oleg and OQ-10 for Montagio; before repeatable Staffy offer: OQ-15.
6. OQ-16, OQ-17, OQ-18 and AG-3 remain open until their explicit product trigger; open does not block unrelated pilot work.

## Fixed Oleg pilot fence

Passing OQ-3 or OQ-5 does not authorize supplier-send. In the controlled Autoparts class, Staffy may create only the exact-version ABCP client/prepared order after customer Confirmation. `supplier_form/send` is absent from the AI/Staffy Action Catalog. A named manager rereads current ABCP version and applicability evidence, acknowledges the task, then approves or blocks with a separate human receipt. Immediately before manually sending to the supplier, the manager performs a final version-matched reread; a mismatch blocks send. A material VIN/vehicle/item/quantity/price/fitment change invalidates the affected customer Confirmation and manager approval. Removing this fence requires a new scoped autonomy decision, Release Manifest and gate evidence.

## Enforcement

- Gate definitions become tracked delivery items; scoped PASS metadata is stored in Capability Evidence, Operating Profile or Release Manifest and may expire.
- The release compiler enumerates every processor/artifact/purpose edge and every requested capability/action; undeclared gate dependencies fail admission.
- Scheduled and event-driven probes evaluate expiry conditions. Provider/account/config change or unavailable probe expires affected instances even if no operator records the change manually.
- A failed/expired gate advances the narrowest Scoped Control generation and invalidates affected queued work before dispatch.
- Recovery drills seed active calls, stale leases, queued and unknown external writes, changed control generations, expired Confirmations and rotated Connector credentials; PASS requires reconciliation without duplicate mutation.
- Product/engineering review lists all gates applicable to the proposed release. A verbal assurance, login, mock, sandbox success or roadmap logo never changes state.
