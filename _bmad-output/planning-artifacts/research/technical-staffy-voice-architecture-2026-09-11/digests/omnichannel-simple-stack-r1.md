# Omnichannel “simple stack” — technical evidence digest (round 1)

**Decision.** Is a model plus tools/skills, STT/TTS, RAG, and MCP sufficient for a universal, deeply integrated business AI employee; and which additional controls are demonstrated necessities rather than speculative platform work?

**Research boundary.** Current official contracts and one first-party production postmortem only; no Staffy project materials were inspected or used. Eight distinct sources, accessed 2026-09-12. No performance claim is made. The only concrete production-failure claim is treated as vendor-reported evidence and paired with a separate current release-control contract before it informs the recommendation.

## Decision answer

**The simple stack is sufficient as a capability substrate, but not as the complete architecture of a deeply integrated business employee.** It supplies the perception/generation loop (voice), reasoning, retrieval, and a standardized way to request actions. It does not, by itself, establish that a requested action was authorized, executed once, completed, reconciled after an ambiguous failure, durably resumed, or safely changed across model/prompt/tool versions.

The minimum additional layer is a small **business execution control plane** around those primitives: authoritative operational reads; an execution gateway that separates proposal from commit; tenant/user/agent authorization; risk-based approval and human handoff; durable action state; explicit retry/idempotency/reconciliation semantics; immutable audit correlation; untrusted-content boundaries; and eval-gated, versioned releases with rollback. These are demonstrated by current vendor contracts or production failure evidence. Building a proprietary universal agent framework, custom speech stack, custom OAuth/MCP protocol, custom vector database, or generalized multi-agent organization before a bounded use case demands it is not demonstrated as necessary.

## Findings

### F1 — A tool call is an intent/request, not proof that an external side effect occurred

- **Claim:** OpenAI’s realtime contract separates a model’s function-tool request from execution: a live `RealtimeSession` tracks and executes tools, and approval can pause a tool run before execution. MCP likewise distinguishes protocol errors from *tool execution* errors. Therefore an emitted tool call cannot be treated as a business commit or success receipt. The application/runtime must own dispatch, validation, authorization, execution, and the result ledger.
- **Source URL:** https://openai.github.io/openai-agents-python/realtime/guide/
- **Publisher:** OpenAI
- **Publication/update date:** Not stated; live documentation retrieved 2026-09-12
- **Accessed:** 2026-09-12
- **Confidence:** High (current official runtime contract)
- **Class:** Architecture contract / tool-calling semantics

### F2 — The speech layer is available, but voice sessions still expose application-owned control seams

- **Claim:** OpenAI’s current realtime architecture accepts incremental audio, streams audio output, tracks local history, performs background tool execution, exposes interruption/disconnection events, and supports tools, approvals, handoffs, guardrails, SIP/telephony, and configurable transcription. This validates reusing a mature voice runtime rather than owning STT/TTS orchestration in the first slice. It also shows that voice transport does not remove the need for tool policy, handoff policy, failure handling, or state outside the live connection.
- **Source URL:** https://openai.github.io/openai-agents-python/realtime/guide/
- **Publisher:** OpenAI
- **Publication/update date:** Not stated; live documentation retrieved 2026-09-12
- **Accessed:** 2026-09-12
- **Confidence:** High (current official runtime contract; no latency/quality claim inferred)
- **Class:** Version/compatibility / voice architecture

### F3 — MCP standardizes connection and invocation; it does not supply the business authorization policy

- **Claim:** MCP authorization is optional at the protocol level, is defined for HTTP transports, and delegates authorization-server implementation beyond the MCP spec. Where used, current MCP requires OAuth 2.1 measures, protected-resource metadata, resource indicators, token audience validation, and forbids token passthrough. Consequently “supports MCP” does not mean “safe for enterprise writes”: the host/server must still enforce tenant, user, delegated-agent, object, and operation policy with least-privilege scopes.
- **Source URL:** https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization
- **Publisher:** Model Context Protocol project
- **Publication/update date:** 2025-11-25 specification
- **Accessed:** 2026-09-12
- **Confidence:** High (normative specification)
- **Class:** Security/control / authorization boundary

### F4 — MCP explicitly leaves confirmation, validation, timeout, sanitization, and audit to implementations

- **Claim:** The current MCP tools specification requires servers to validate inputs, implement access controls and rate limits, and sanitize outputs; it advises clients to show inputs and obtain confirmation for sensitive operations, validate results, impose timeouts, and log tool use for audit. Tool annotations are untrusted unless the server is trusted. These are not optional “enterprise polish” for consequential tools; they are the protocol’s documented implementation boundary.
- **Source URL:** https://modelcontextprotocol.io/specification/2025-11-25/server/tools
- **Publisher:** Model Context Protocol project
- **Publication/update date:** 2025-11-25 specification
- **Accessed:** 2026-09-12
- **Confidence:** High (normative and recommended controls in current specification)
- **Class:** Security/control / audit / tool poisoning

### F5 — MCP elicitation is a user-interaction primitive, not an authorization or completion guarantee

- **Claim:** MCP elicitation lets a server ask for structured input or send a user to an out-of-band URL. Form mode must not collect passwords, API keys, access tokens, or payment credentials; URL mode is separate from MCP client authorization. An `accept` response means the user consented to open the interaction, not that the interaction completed, and clients should retain manual retry/cancel controls if completion notification never arrives. Business workflows therefore need their own pending/complete/cancelled state and verified outcome.
- **Source URL:** https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation
- **Publisher:** Model Context Protocol project
- **Publication/update date:** 2025-11-25 specification
- **Accessed:** 2026-09-12
- **Confidence:** High (normative specification)
- **Class:** Architecture contract / elicitation / human interaction

### F6 — RAG is knowledge retrieval, not a substitute for live operational truth

- **Claim:** Microsoft defines RAG as retrieve → augment → generate over an index/data store, with quality depending on preparation and retrieval configuration. Irrelevant or incomplete retrieval can still produce incomplete or inaccurate answers. The safe architecture is therefore: use RAG for policies, playbooks, historical knowledge, and other document-like context; query the authoritative transactional system for mutable facts such as availability, order status, balances, permissions, and reservations; validate again immediately before a write.
- **Source URL:** https://learn.microsoft.com/en-us/azure/foundry/concepts/retrieval-augmented-generation
- **Publisher:** Microsoft
- **Publication/update date:** Updated 2026-05-20
- **Accessed:** 2026-09-12
- **Confidence:** High for RAG limits; high for the source-of-truth architectural inference
- **Class:** Architecture pattern / knowledge versus operational state

### F7 — Retrieved documents and tool metadata cross an untrusted-content boundary

- **Claim:** Microsoft explicitly says to treat retrieved content as untrusted input because documents/passages can carry prompt injection, while MCP says tool annotations are untrusted unless they come from trusted servers and requires output sanitization/validation. A prompt-only defense is therefore insufficient. The minimum control is structural: identify provenance, enforce retrieval-time ACLs, separate untrusted context from instructions, constrain tool capability and data egress, and require a guarded commit path for sensitive writes.
- **Source URLs:** https://learn.microsoft.com/en-us/azure/foundry/concepts/retrieval-augmented-generation ; https://modelcontextprotocol.io/specification/2025-11-25/server/tools
- **Publishers:** Microsoft; Model Context Protocol project
- **Publication/update dates:** 2026-05-20; 2025-11-25 specification
- **Accessed:** 2026-09-12
- **Confidence:** High (two current primary sources define both sides of the boundary)
- **Class:** Security/control / prompt injection / tool poisoning

### F8 — Durable workflow semantics and idempotency are required for side effects; “exactly once” is not implied

- **Claim:** AWS’s current durable-execution contract states that replay/retry can execute an operation more than once; default at-least-once is safe only for idempotent work; at-most-once per attempt still does not guarantee exactly once across a workflow; and external idempotency keys must be generated durably and reused across attempts. The first slice therefore needs an action record and stable idempotency key before executing a write, plus bounded retry and an `unknown` outcome that triggers reconciliation rather than blind replay.
- **Source URL:** https://docs.aws.amazon.com/durable-execution/patterns/best-practices/idempotency/
- **Publisher:** Amazon Web Services
- **Publication/update date:** Not stated; current Durable Execution SDK documentation retrieved 2026-09-12
- **Accessed:** 2026-09-12
- **Confidence:** High (current official execution semantics)
- **Class:** Operational semantics / durability / retries / reconciliation

### F9 — Human approval and handoff must be attached to the executable action, not merely conversational text

- **Claim:** OpenAI realtime tool approval pauses the tool run until explicit approve/reject, and its handoff mechanism transfers the live conversation to a specialist. MCP recommends showing tool inputs before sensitive calls, and MCP elicitation distinguishes consent from completion. Together these contracts support a concrete boundary: the user/operator reviews the exact normalized action payload; approval is correlated to that action and expires on material changes; a human receives context plus action status; and handoff does not erase the pending workflow.
- **Source URLs:** https://openai.github.io/openai-agents-python/realtime/guide/ ; https://modelcontextprotocol.io/specification/2025-11-25/server/tools ; https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation
- **Publishers:** OpenAI; Model Context Protocol project
- **Publication/update dates:** Not stated (live docs); 2025-11-25; 2025-11-25
- **Accessed:** 2026-09-12
- **Confidence:** High for mechanisms; medium-high for the exact payload-binding design, which is an inference from those boundaries
- **Class:** Architecture pattern / human-in-the-loop / handoff

### F10 — Evals, version pinning, canarying, and rollback are an operational release system, not optional model tuning

- **Claim:** OpenAI’s prompt-management contract provides immutable published versions, version pinning, linked repeatable evals, side-by-side comparison, and one-click rollback, and instructs users to rerun evals after every publish. Anthropic’s 2025 production postmortem reports three infrastructure bugs that intermittently degraded responses, says noisy evaluations delayed diagnosis, and records rollback plus new detection tests and continuous production evaluations as remediations. The supported conclusion is not that any one provider is unreliable; it is that prompt/model/tool changes require versioned configuration, representative end-to-end evals, staged exposure, observability, and a tested rollback path.
- **Source URLs:** https://help.openai.com/en/articles/9824968 ; https://www.anthropic.com/engineering/a-postmortem-of-three-recent-issues
- **Publishers:** OpenAI; Anthropic
- **Publication/update dates:** Updated 2026-08; published 2025-09-17
- **Accessed:** 2026-09-12
- **Confidence:** High for the control pattern (independent vendors); medium for the incident’s exact impact details because they are first-party self-report
- **Class:** Production incident / evals / versioning / rollback

### F11 — Comparison of mature contracts: convergence is on a thin agent loop plus an external control plane

- **Claim:** Across the three mature contracts inspected—OpenAI Agents/Realtime, MCP, and AWS Durable Execution—the responsibilities divide consistently:
  - **OpenAI Agents/Realtime:** voice/session loop, tool dispatch surface, interruptions, guardrails, approvals, and conversational/specialist handoff;
  - **MCP:** interoperable discovery/invocation, OAuth-oriented transport authorization, elicitation, typed results/errors, and documented host/server safety duties;
  - **AWS Durable Execution:** checkpoint/replay semantics, explicit at-least/at-most-once choices, retries, and stable idempotency across side effects.
  No inspected contract collapses these into “the model handles it.” The interoperable architecture is a replaceable conversational agent over a durable, policy-enforcing business execution layer.
- **Source URLs:** https://openai.github.io/openai-agents-python/realtime/guide/ ; https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization ; https://modelcontextprotocol.io/specification/2025-11-25/server/tools ; https://docs.aws.amazon.com/durable-execution/patterns/best-practices/idempotency/
- **Publishers:** OpenAI; Model Context Protocol project; Amazon Web Services
- **Publication/update dates:** Not stated (live docs); 2025-11-25; 2025-11-25; not stated (current docs)
- **Accessed:** 2026-09-12
- **Confidence:** High (cross-vendor architectural inference from current contracts)
- **Class:** Cross-vendor architecture pattern

## What is demonstrated necessity vs. overengineering?

### Demonstrated necessities

1. **Proposal/commit separation.** Normalize a model tool request into an application-owned action; only the execution gateway can commit it.
2. **Authoritative operational reads.** Read volatile business facts from the owning API/database service; use RAG for document knowledge and explanation.
3. **Identity and least privilege.** Carry tenant, user, agent, scopes, resource audience, and delegation context through every call; do not pass bearer tokens between services.
4. **Risk-based approval and human handoff.** Require exact-payload approval for consequential or ambiguous actions, preserve the pending action through handoff, and fail closed on malformed inputs.
5. **Durable action state.** Persist correlation ID, idempotency key, normalized input, policy decision, approval, attempts, external receipt, terminal/unknown state, and reconciliation result.
6. **Explicit retry semantics.** Retry safe/idempotent reads and writes with limits/backoff; never blindly retry a non-idempotent side effect after an ambiguous timeout; reconcile first.
7. **Audit and observability.** Correlate conversation/turn, model and prompt version, retrieval provenance, tool schema/version, principal, policy/approval, request, result, and external receipt.
8. **Untrusted-content isolation.** Treat retrieved documents, web text, MCP annotations, and tool outputs as data, not instructions; enforce ACLs, output validation, egress limits, and small tool scopes.
9. **Release controls.** Pin prompts/models/tool schemas for production workflows; run end-to-end regression evals on representative transcripts and side-effect traces; canary changes and retain one-step rollback.

### Not demonstrated as first-slice necessities

- A proprietary general-purpose agent framework or multi-agent hierarchy.
- A custom STT/TTS stack when a realtime/voice provider meets the target channel’s requirements.
- A custom MCP authorization protocol or homegrown OAuth implementation.
- A bespoke vector database or universal enterprise knowledge graph before retrieval quality/scale demands it.
- General event sourcing for every interaction; an append-only action/audit record and durable workflow checkpoints are enough initially.
- Automatic execution of every discovered MCP tool, a broad tool marketplace, or permanent blanket approvals.
- “Exactly once” claims across arbitrary external systems; explicit semantics, idempotency, receipts, and reconciliation are the defensible contract.
- Cross-channel omniscient memory assembled from transcripts without purpose limits, retention rules, provenance, and subject-level access controls.

## Ponytail classification

### Must in first slice

- One bounded business workflow and one primary channel.
- Reused voice/runtime integration with interruption handling and clean human transfer.
- Operational source-of-truth adapter plus RAG only for relevant document knowledge.
- Tool/action gateway with schema validation, server-side authorization, risk tier, timeout, audit, and allowlist.
- Durable action record with correlation/idempotency key, explicit state (`proposed`, `awaiting_approval`, `executing`, `succeeded`, `failed`, `unknown`, `reconciled`), bounded retries, and a reconciliation path.
- Exact-action approval for consequential writes and a human handoff packet containing intent, known facts, unresolved items, and action status.
- Retrieval/tool-output trust boundaries and least-privilege credentials.
- Version-pinned prompt/model/tool configuration, a small end-to-end eval suite, canary release, and tested rollback.

### Thin contract now

- Provider-neutral `ConversationSession`, `KnowledgeQuery`, `ActionProposal`, `ActionReceipt`, `Approval`, `Handoff`, and `AuditEvent` envelopes.
- A principal/delegation context carried across RAG, MCP, native APIs, and handoff.
- Per-tool metadata for read/write, reversibility, risk, idempotency support, timeout, retry policy, and reconciliation method—owned by the host, not trusted from remote annotations alone.
- Provenance/freshness fields on retrieved knowledge and a rule that volatile facts bypass document indexes.
- Version tuple on every run: agent instructions/prompt, model, tool schema/adapter, policy, retrieval/index, and eval-suite version.

### Reuse provider/framework

- Realtime audio transport, turn detection, STT/TTS, SIP/telephony, and live-session mechanics.
- Agent loop, function-tool plumbing, ordinary conversational state, tool-approval interruption, guardrails, and specialist handoff.
- MCP protocol libraries plus standard OAuth 2.1/OIDC infrastructure.
- Managed search/vector indexing and document-level access filtering.
- A durable workflow engine/checkpointing framework once the workflow spans process restarts, hours/days, callbacks, or multiple side effects.
- Existing tracing/telemetry and prompt/eval/version-management facilities where they satisfy retention and access requirements.

### Defer

- Universal multi-agent organization, autonomous planner, or learned skill routing.
- Custom speech models, custom protocol transports, custom identity provider, custom vector engine, or generalized workflow engine.
- Broad autonomous write access, blanket tool approvals, self-modifying prompts/tools, and automatic cross-tenant memory.
- Complex compensation/event-sourcing machinery until the first bounded workflows expose a real need beyond action log + reconciliation.
- Provider abstraction below the thin contracts unless a second provider is actually being integrated.

## Source inventory (8 distinct sources)

1. OpenAI, **Realtime agents guide** — https://openai.github.io/openai-agents-python/realtime/guide/ — live docs, date not stated; accessed 2026-09-12.
2. OpenAI, **Prompt management in Playground** — https://help.openai.com/en/articles/9824968 — updated 2026-08; accessed 2026-09-12.
3. Model Context Protocol project, **Authorization (2025-11-25)** — https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization — 2025-11-25; accessed 2026-09-12.
4. Model Context Protocol project, **Tools (2025-11-25)** — https://modelcontextprotocol.io/specification/2025-11-25/server/tools — 2025-11-25; accessed 2026-09-12.
5. Model Context Protocol project, **Elicitation (2025-11-25)** — https://modelcontextprotocol.io/specification/2025-11-25/client/elicitation — 2025-11-25; accessed 2026-09-12.
6. Microsoft, **Retrieval augmented generation (RAG) and indexes in Microsoft Foundry** — https://learn.microsoft.com/en-us/azure/foundry/concepts/retrieval-augmented-generation — updated 2026-05-20; accessed 2026-09-12.
7. Amazon Web Services, **Idempotency and retries — AWS Durable Execution SDK** — https://docs.aws.amazon.com/durable-execution/patterns/best-practices/idempotency/ — live docs, date not stated; accessed 2026-09-12.
8. Anthropic, **A postmortem of three recent issues** — https://www.anthropic.com/engineering/a-postmortem-of-three-recent-issues — published 2025-09-17; accessed 2026-09-12.

## Leads and limits

- No trustworthy, recent, independently verified public production postmortem was found for a general business voice agent executing real-world transactions. Vendor marketing was excluded as proof of reliability or performance.
- Public contracts support the control boundaries above, but they do not prove that any single vendor stack delivers a “universal employee” across arbitrary domains.
- A follow-up should test these contracts against one concrete business workflow: enumerate every read/write, ambiguous outcome, approval, handoff, freshness requirement, and compensating/reconciliation action, then confirm the chosen providers expose the necessary hooks.
