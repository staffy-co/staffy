---
title: Staffy Architecture Consistency Review R4
status: review-complete
reviewed: 2026-09-15
---

# Architecture consistency review — R4

## Verdict

**PASS — no remaining material consistency blockers.**

- The approval contract now matches `allow_role_coincidence`: all required roles remain mandatory, while one principal may hold them only when the machine definition explicitly permits it.
- OQ-12 and OQ-13B human deadlines now match the authoritative machine definitions and execution order: both block controlled-real-customer voice and later stages; the pre-PASS state is synthetic/closed-account only.
- Both machine registries remain schema-valid: 25 unique definitions and 35 unique instances, with valid references and no missing dependencies.
- The sole PASS instance, `oq-13a-montagio`, satisfies its approver contract and its evidence file exists with the recorded SHA-256.
- `ARCHITECTURE-SPINE.md` remains `draft` pending final close, and deterministic spine lint reports zero findings.

This consistency lens approves finalization.
