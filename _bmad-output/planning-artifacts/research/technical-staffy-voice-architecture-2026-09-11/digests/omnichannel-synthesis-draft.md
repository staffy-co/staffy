# Omnichannel + Ponytail synthesis draft

**Status:** awaiting Chatwoot round-2 verification.

## Direct answers

1. **Pipecat and analogs are in scope as pattern libraries and optional runtime adapters.** Reuse typed events, priority control lanes, transport separation, explicit turn state, heard-output context, cancellable tool jobs, deterministic handoff, session-worker isolation, and event-aligned tests/telemetry. Do not copy Pipecat's exact frame tree or make any framework's room/job/pipeline object the Staffy business model.
2. **`model + tools/skills + STT/TTS + RAG + MCP` is a sufficient capability substrate, not a complete accountable business runtime.** Tool invocation does not prove authorization or completion; MCP explicitly delegates access control, validation, confirmation, timeouts, sanitization, and audit; RAG is not the source of live prices/orders/availability; external writes need durable idempotency and reconciliation.
3. **The earlier Staffy shape is directionally right but over-separated for two pilots.** Keep the invariants, collapse the deployment units: Action Gateway becomes an in-process Go module; Operation Ledger becomes one append-only mutations table; versioning becomes immutable config hash + small eval gate; write lease becomes atomic conversation ownership; the canonical channel model is a small envelope, not a universal messaging platform.
4. **No verified universal third-party API covers the target Russian channel set.** Use one Staffy envelope/outbox and thin official adapters. Jivo is an optional existing-inbox bridge. A self-hosted Chatwoot system-of-engagement may remove web/Telegram/inbox work, but needs a current one-day spike before selection. MAX retains a small stateful bridge.

## Ponytail disposition

| Concern | Pilot form | Later trigger |
| --- | --- | --- |
| Conversation/runtime orchestration | Reuse provider/framework loop behind Staffy contract | Own more only on measured capability/latency/residency gap |
| Action Gateway | One in-process module: schemas, auth, risk, approval, execute | Separate service when multiple independent runtimes consume it |
| Operation Ledger | One append-only table for mutating actions and unknown outcomes | Workflow engine/event sourcing only after multi-step durable workflows demand it |
| Config versioning | Immutable `runtime_config_hash`, model/tool/policy versions, small eval suite, rollback | Dedicated registry/canary platform after many tenants/teams |
| RAG | Managed/simple retrieval for documents; provenance and ACL | Custom vector/knowledge platform only on scale/quality evidence |
| MCP | Connector protocol where useful, never authorization authority | Marketplace/discovery after enough reusable connectors exist |
| Channel core | Minimal ingress envelope, outbox, dedupe, attachment refs, atomic actor ownership | Rich canonical model/SDK after second real channel family proves need |
| Human inbox | Existing Jivo; test self-hosted Chatwoot alternative | Build Staffy inbox only if external inboxes block product differentiation |
| Speech/media | Managed STT/TTS/realtime and replaceable edge | Thin Go media path on measured gates; full owned stack much later |

## Minimal channel contract

Ingress: `tenant`, `connection`, `external_conversation_id`, `external_message_id`, `sender`, `occurred_at`, `content_parts`, `reply_to`, `raw_event_ref`, `capabilities`.

Egress: `conversation`, `idempotency_key`, `content_parts`, `reply_to`, `required_capability`, `expiry`, `handoff_owner`.

State owned by Staffy in pilot: dedupe receipt; outbound attempt/status; channel-local identity link; attachment pointer; bot/human owner; mutating business operation receipt. Never synthesize delivered/read states that the provider did not emit.

## Rollout candidate

- Tier 0: first-party web widget + common protocol; install by script in Django or ABCP HTML/footer block.
- Tier 1: Telegram, MAX, Jivo; Android/iOS use the same API before a public SDK.
- Tier 2: VK and Avito after real-account contract tests.
- Deferred: WhatsApp Business Platform until Russia-specific official/BSP eligibility and a real webhook test are confirmed.

## Decision still open

Whether self-hosted Chatwoot should own pilot conversation/inbox state or remain only a source of patterns. Resolve with current-doc verification and a one-day spike; do not let this choice alter Staffy's business-action boundary.
