# Graph Report - staffy  (2026-09-28)

## Corpus Check
- 234 files · ~400,677 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 15 file(s) not represented in the graph (top: .excalidraw 10, (none) 4, .css 1)

## Summary
- 1284 nodes · 3325 edges · 81 communities (63 shown, 18 thin omitted)
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 150 edges (avg confidence: 0.87)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `623bf027`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- properties
- properties
- properties
- properties
- Staffy Product Requirements Document
- type
- Staffy Montagio Consilium Synthesis
- Omnichannel Synthesis Draft
- Staffy Competitive Research
- context.Context
- Staffy Visual System
- Common Platform Core
- gate-definitions.schema.json
- gate-instances.schema.json
- Montagio Slice C UX Patterns
- AI Employee Readiness Workspace
- Voice Runtime Maturity Round 2
- Operations Verification R2
- Staffy Hardened Idea
- Go Voice-Core Latency and Economics Analysis
- Russian Competitor Qualification
- Staffy Sprint Status
- Action Gateway
- Employee Not Technology Metaphor
- spec-1-3-guided-employee-draft.md
- store/membership.go
- net/http.Request
- Service
- 00001_identity_tenant.sql
- spec-1-1-tenant-workspace.md
- @playwright/test
- gate-instance.schema.json
- Epic 1 Context: Защищённая самостоятельная настройка ИИ-сотрудника
- dependencies
- spec-1-4-versioned-domain-pack.md
- 00001_employee_draft.sql
- spec-1-2-tenant-membership.md
- go_pkg_github_com_artemiosu_staffy_internal_platform_id
- app/behavior_test.go
- Context
- go_pkg_embed
- github.com/artemiosu/staffy
- api/voice.go
- Principal
- identity.tenants
- KnowledgeSource
- validate.go
- time.Time
- testing.T
- verifierDatabase
- retentionTx
- identity.memberships
- domain/voice.go
- .Create
- PackManifest
- UUID
- spec-1-5-knowledge-sources.md
- employee.knowledge_sources
- employee.drafts
- spec-1-6-safe-behavior-configuration.md
- spec-1-7-voice-and-pronunciation.md
- spec-1-5-safe-source-ingestion.md
- employee.behavior_versions
- employee.voice_versions
- Draft
- actor_id
- approved_at
- role
- sha256
- tested_at
- uri
- employee.knowledge_sources
- employee.knowledge_versions
- employee.audit
- employee.audit
- employee.audit
- employee.voice_previews

## God Nodes (most connected - your core abstractions)
1. `Principal` - 79 edges
2. `KnowledgeSource` - 45 edges
3. `FromContext()` - 36 edges
4. `employeeFixture` - 34 edges
5. `Draft` - 33 edges
6. `Context` - 32 edges
7. `Parse()` - 31 edges
8. `Membership` - 26 edges
9. `Server` - 26 edges
10. `KnowledgeVersion` - 22 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `New()`  [INFERRED]
  cmd/staffy-core/main.go → internal/identity/app/service.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/staffy-core/main.go → internal/adapters/authdev/fixed.go
- `run()` --calls--> `NewWithTestTTS()`  [EXTRACTED]
  cmd/staffy-core/main.go → internal/employee/app/service.go
- `run()` --calls--> `NewPostgres()`  [EXTRACTED]
  cmd/staffy-core/main.go → internal/employee/store/postgres.go
- `run()` --calls--> `NewPostgres()`  [EXTRACTED]
  cmd/staffy-core/main.go → internal/identity/store/postgres.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Staffy UX Review Suite** — _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_review_accessibility_r2_accessibility_review_r2, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_review_rubric_r2_spine_pair_review_r2, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_review_safety_operations_r2_safety_operations_review_r2, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_review_editorial_editorial_review, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_validation_report_final_ux_validation [EXTRACTED 1.00]
- **Common Core Validated by Both Pilot Domains** — _bmad_output_planning_artifacts_epics_common_platform_core, _bmad_output_planning_artifacts_epics_epic_4_auto_parts_expert, _bmad_output_planning_artifacts_epics_epic_5_montagio_services_expert [EXTRACTED 1.00]
- **Consilium Evidence Methods** — _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_elicitation_01_assumption_audit_assumption_audit, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_elicitation_02_cascading_failure_simulation_cascading_failure_simulation, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_elicitation_03_user_persona_focus_group_persona_focus_group, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_elicitation_04_customer_support_theater_customer_support_theater, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_elicitation_05_delphi_method_delphi_method [EXTRACTED 1.00]
- **Delphi Expert Panel** — _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_delphi_round1_domain_delphi_domain_round_1, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_delphi_round1_marketplace_delphi_marketplace_round_1, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_delphi_round1_ux_delphi_ux_round_1, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_delphi_round2_domain_delphi_domain_round_2, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_delphi_round2_marketplace_delphi_marketplace_round_2, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_delphi_round2_ux_delphi_ux_round_2 [EXTRACTED 1.00]
- **Go Voice-Core Decision Evidence** — _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_go_decision_media_scope_r1_media_scope_decision, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_go_decision_pipecat_scale_r1_pipecat_scale_decision, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_go_decision_red_team_go_decision_red_team [EXTRACTED 1.00]
- **Omnichannel Architecture Evidence** — _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_omnichannel_ru_channels_r1_russian_channel_architecture, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_omnichannel_simple_stack_r1_simple_stack_assessment, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_omnichannel_voice_patterns_r1_voice_runtime_patterns, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_omnichannel_synthesis_draft_omnichannel_synthesis [EXTRACTED 1.00]
- **Staffy Architecture Review Closure** — _bmad_output_planning_artifacts_architecture_architecture_staffy_2026_09_13_review_consistency_architecture_consistency_review, _bmad_output_planning_artifacts_architecture_architecture_staffy_2026_09_13_review_consistency_r4_architecture_consistency_pass, _bmad_output_planning_artifacts_architecture_architecture_staffy_2026_09_13_review_safety_implementation_architecture_safety_review, _bmad_output_planning_artifacts_architecture_architecture_staffy_2026_09_13_review_safety_implementation_r3_architecture_safety_pass [EXTRACTED 1.00]
- **Staffy Six-Epic Delivery Decomposition** — _bmad_output_planning_artifacts_epics_epic_1_protected_self_service_setup, _bmad_output_planning_artifacts_epics_epic_2_unified_customer_service, _bmad_output_planning_artifacts_epics_epic_3_safe_business_actions, _bmad_output_planning_artifacts_epics_epic_4_auto_parts_expert, _bmad_output_planning_artifacts_epics_epic_5_montagio_services_expert, _bmad_output_planning_artifacts_epics_epic_6_governed_autonomy [EXTRACTED 1.00]
- **Staffy Montagio Guardrails** — _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_conversation_ux_conversational_ux_consilium, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_marketplace_ops_marketplace_operations, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_service_domain_service_domain, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_consilium_synthesis_staffy_montagio_synthesis [EXTRACTED 1.00]
- **Staffy Platform Proof System** — _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_prd_managed_ai_employee_platform, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_prd_controlled_autonomy, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_prd_business_action_safety, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_prd_two_domain_proof [EXTRACTED 1.00]
- **Staffy PRD Reconciliation Inputs** — _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_reconcile_forge_research_consilium_forge_research_reconciliation, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_reconcile_original_chat_original_chat_reconciliation, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_reconcile_product_brief_product_brief_reconciliation, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_prd_staffy_prd [EXTRACTED 1.00]
- **Staffy PRD Review Gate** — _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_review_conversation_ux_conversation_ux_review, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_review_product_coherence_product_coherence_review, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_review_rubric_prd_quality_review, _bmad_output_planning_artifacts_prds_prd_staffy_2026_09_12_review_resolution_review_resolution [EXTRACTED 1.00]
- **Staffy Key Operational Surfaces** — _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_mockups_key_today_owner_owner_today_workspace, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_mockups_key_employee_workspace_employee_readiness_workspace, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_mockups_key_autoparts_order_review_autoparts_order_review [INFERRED 0.85]
- **Staffy Vertical and Architecture Strategy** — _bmad_output_planning_artifacts_research_competitive_staffy_2026_09_11_research_vertical_wedge_decision, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_research_owned_control_plane_replaceable_media, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_research_action_gateway_operation_ledger [INFERRED 0.85]
- **Staffy UX Spine and Visual Evidence** — _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_design_staffy_visual_system, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_experience_staffy_experience_specification, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_color_themes_5_color_theme_exploration, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_directions_4_visual_direction_exploration, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_ia_proposal_v1_information_architecture_proposal, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_key_autoparts_order_review_autoparts_order_review_mock [INFERRED 0.95]
- **Technical Voice Architecture Verification** — _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_verification_operations_r2_operations_verification_r2, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_verification_operations_operations_verification, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_verification_runtime_r2_runtime_verification_r2, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_verification_runtime_runtime_verification [INFERRED 0.95]
- **Competitive Research Evidence Program** — _bmad_output_planning_artifacts_research_competitive_staffy_2026_09_11_digests_landscape_leads_r1_lead_voice_agent_landscape, _bmad_output_planning_artifacts_research_competitive_staffy_2026_09_11_digests_offers_pricing_r1_1_offer_pricing_teardown, _bmad_output_planning_artifacts_research_competitive_staffy_2026_09_11_digests_customer_voice_r1_1_customer_voice, _bmad_output_planning_artifacts_research_competitive_staffy_2026_09_11_research_competitive_research [INFERRED 0.95]
- **Montagio Protected Continuation Evidence** — _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_source_montagio_conversation_montagio_conversation_ux_evidence, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_source_montagio_safety_commerce_montagio_safety_and_commerce_invariants, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_source_montagio_ux_montagio_slice_c_ux_patterns, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13_mockups_key_montagio_continuation_protected_montagio_continuation [INFERRED 0.95]
- **Pilot Architecture Decision Inputs** — _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_integration_reliability_r2_1_integration_reliability_round_2, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_operations_tco_ecosystem_r2_1_operations_tco_round_2, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_provider_eligibility_components_r1_lead_provider_eligibility, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_runtime_maturity_r2_1_runtime_maturity_round_2 [INFERRED 0.95]
- **Staffy Controlled Action Chain** — _bmad_output_forge_staffy_vertical_voice_agent_forged_idea_controlled_business_actions, _bmad_output_planning_artifacts_architecture_architecture_staffy_2026_09_13_architecture_spine_action_gateway, _bmad_output_planning_artifacts_architecture_architecture_staffy_2026_09_13_gate_registry_fail_closed_admission [INFERRED 0.95]
- **Staffy UX Evidence Synthesis** — _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_source_prd_final_prd_ux_requirements, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_source_research_research_derived_ux_requirements, _bmad_output_planning_artifacts_ux_designs_ux_staffy_2026_09_13__working_source_vision_founder_ux_vision [INFERRED 0.95]
- **Technical Build-Buy Decision Evidence** — _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_brief_technical_research_brief, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_research_voice_architecture_research, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_go_decision_latency_economics_r1_go_latency_economics, _bmad_output_planning_artifacts_research_technical_staffy_voice_architecture_2026_09_11_digests_go_decision_latency_economics_verification_go_latency_economics_verification [INFERRED 0.95]

## Communities (81 total, 18 thin omitted)

### Community 0 - "properties"
Cohesion: 0.06
Nodes (34): additionalProperties, type, const, items, minItems, type, uniqueItems, enum (+26 more)

### Community 1 - "properties"
Cohesion: 0.06
Nodes (34): minLength, type, minLength, type, minLength, type, minLength, type (+26 more)

### Community 2 - "properties"
Cohesion: 0.11
Nodes (19): minLength, type, format, type, enum, properties, enum, minLength (+11 more)

### Community 3 - "properties"
Cohesion: 0.10
Nodes (20): type, minimum, type, pattern, type, pattern, type, properties (+12 more)

### Community 4 - "Staffy Product Requirements Document"
Cohesion: 0.05
Nodes (46): Staffy Architecture Party Decisions, Staffy Architecture Decision Log, Staffy Architecture Map, Go Modular Monolith, Staffy Architecture Spine, Montagio Best-Effort Pilot Policy, State and Authority Matrix Rule, Executable Architecture Gate Registry (+38 more)

### Community 5 - "type"
Cohesion: 0.17
Nodes (16): items, items, minItems, type, items, items, type, items (+8 more)

### Community 6 - "Staffy Montagio Consilium Synthesis"
Cohesion: 0.09
Nodes (22): Conversational UX Consilium, Delphi Domain Round 1, Delphi Marketplace Round 1, Delphi UX Round 1, Delphi Domain Round 2, Delphi Marketplace Round 2, Delphi UX Round 2, Assumption Audit (+14 more)

### Community 7 - "Omnichannel Synthesis Draft"
Cohesion: 0.11
Nodes (19): Go Media-Core Scope Decision, Staged Media Infrastructure Adoption, Media Scope Verification, Pipecat Scaling Decision, Pipecat Scaling Verification, Go Voice-Core Red Team, Integration Reliability Round 1, Reliable Business Action Contract (+11 more)

### Community 8 - "Staffy Competitive Research"
Cohesion: 0.11
Nodes (18): Auto-Parts Vertical Wedge, Voice-Agent Customer Voice, Competitive Research Citation Check, Voice-Agent Offer and Pricing Teardown, Voice-Agent Market Position Trajectory, Vertical Demand Verification, Product and Pricing Verification, Vertical Voice-Agent Entrants (+10 more)

### Community 9 - "context.Context"
Cohesion: 0.15
Nodes (9): packIdentity, context.Context, fakeMembers, UnavailableScanner, MembershipCommand, validMutation(), Membership, WithID() (+1 more)

### Community 10 - "Staffy Visual System"
Cohesion: 0.17
Nodes (16): Staffy UX Decision Memory, Five Light Color Systems, Four Visual Directions, Staffy Information Architecture Proposal, Autoparts Order Review Key Screen, Staffy Visual System, Staffy Experience Specification, Accessibility Review (+8 more)

### Community 11 - "Common Platform Core"
Cohesion: 0.14
Nodes (15): Epic 1 Backlog Status, Epic 2 Backlog Status, Epic 3 Backlog Status, Epic 4 Backlog Status, Epic 5 Backlog Status, Epic 6 Backlog Status, Common Platform Core, Epic 1: Protected Self-Service AI Employee Setup (+7 more)

### Community 12 - "gate-definitions.schema.json"
Cohesion: 0.14
Nodes (13): additionalProperties, items, minItems, type, $id, $ref, properties, definitions (+5 more)

### Community 13 - "gate-instances.schema.json"
Cohesion: 0.14
Nodes (13): additionalProperties, $id, items, minItems, type, $ref, properties, instances (+5 more)

### Community 14 - "Montagio Slice C UX Patterns"
Cohesion: 0.15
Nodes (13): Protected Montagio Continuation, Owner Today Workspace, Montagio Conversation UX Evidence, Montagio Safety and Commerce Invariants, Montagio Slice C UX Patterns, Final PRD UX Requirements, Single Response Owner, Five Light Color Systems (+5 more)

### Community 15 - "AI Employee Readiness Workspace"
Cohesion: 0.33
Nodes (6): AI Employee Readiness Workspace, Reference UX Patterns, Visible AI Employee Lifecycle, Capability Readiness Model, Research-Derived UX Requirements, Published AI Employee Readiness Workspace

### Community 16 - "Voice Runtime Maturity Round 2"
Cohesion: 0.40
Nodes (5): Operations TCO Ecosystem Round 1, Operations TCO Residency Round 2, Russian Provider Eligibility, Voice Runtime Maturity Round 1, Voice Runtime Maturity Round 2

### Community 17 - "Operations Verification R2"
Cohesion: 0.40
Nodes (5): Operations Verification, Operations Verification R2, Runtime Verification R2, Runtime Verification, Imported AI Search Conversation

### Community 18 - "Staffy Hardened Idea"
Cohesion: 0.50
Nodes (4): Staffy Forge Decisions, Staffy Hardened Forge Report, Controlled Business Actions, Staffy Hardened Idea

### Community 19 - "Go Voice-Core Latency and Economics Analysis"
Cohesion: 0.50
Nodes (4): Go Voice-Core Claims from Source Chat, Go Decision Final Semantic Check, Go Voice-Core Latency and Economics Analysis, Go Latency and Economics Verification

### Community 20 - "Russian Competitor Qualification"
Cohesion: 0.67
Nodes (3): Russian Competitor Qualification, Voice-Agent Competitive Landscape Leads, Competitor Qualification Risk Verification

### Community 24 - "spec-1-3-guided-employee-draft.md"
Cohesion: 0.20
Nodes (9): Boundaries & Constraints, Code Map, I/O & Edge-Case Matrix, Implementation Notes, Intent, Review Triage Log, Spec Change Log, Tasks & Acceptance (+1 more)

### Community 25 - "store/membership.go"
Cohesion: 0.15
Nodes (26): memberInput, sensitive(), TestGrantLimits(), TestReasonCodesAreBounded(), AllowedScopes(), AllScopes(), containsAllScopes(), ReasonCode (+18 more)

### Community 26 - "net/http.Request"
Cohesion: 0.06
Nodes (50): KnowledgeIngestService, KnowledgeService, envOr(), loopbackAddress(), main(), run(), TestLoopbackAddressVariants(), TestRunRejectsDevelopmentResolverOutsideLoopbackBeforeDatabaseAccess() (+42 more)

### Community 27 - "Service"
Cohesion: 0.18
Nodes (11): fakeIdentity, Clock, CreateTenantCommand, CreateTenantResult, IDGenerator, Service, hasLetterOrNumber(), membershipScope() (+3 more)

### Community 28 - "00001_identity_tenant.sql"
Cohesion: 0.62
Nodes (6): identity, identity.idempotency_records, identity.memberships, identity.outbox_events, identity.principals, identity.tenants

### Community 29 - "spec-1-1-tenant-workspace.md"
Cohesion: 0.20
Nodes (9): Boundaries & Constraints, Code Map, I/O & Edge-Case Matrix, Implementation Notes, Intent, Review Triage Log, Spec Change Log, Tasks & Acceptance (+1 more)

### Community 30 - "@playwright/test"
Cohesion: 0.12
Nodes (7): @playwright/test, devDependencies, @playwright/test, name, private, scripts, test

### Community 31 - "gate-instance.schema.json"
Cohesion: 0.22
Nodes (8): additionalProperties, allOf, $id, required, $schema, title, type, type

### Community 32 - "Epic 1 Context: Защищённая самостоятельная настройка ИИ-сотрудника"
Cohesion: 0.25
Nodes (7): Cross-Story Dependencies, Epic 1 Context: Защищённая самостоятельная настройка ИИ-сотрудника, Goal, Requirements & Constraints, Stories, Technical Decisions, UX & Interaction Patterns

### Community 33 - "dependencies"
Cohesion: 0.24
Nodes (7): type, uniqueItems, items, type, uniqueItems, dependencies, permits

### Community 34 - "spec-1-4-versioned-domain-pack.md"
Cohesion: 0.18
Nodes (10): Boundaries & Constraints, Code Map, Design Notes, I/O & Edge-Case Matrix, Implementation Notes, Intent, Review Triage Log, Spec Change Log (+2 more)

### Community 35 - "00001_employee_draft.sql"
Cohesion: 0.53
Nodes (5): employee.audit, employee.drafts, employee.idempotency_records, employee.outbox_events, employee

### Community 36 - "spec-1-2-tenant-membership.md"
Cohesion: 0.20
Nodes (9): Boundaries & Constraints, Code Map, I/O & Edge-Case Matrix, Implementation Notes, Intent, Review Triage Log, Spec Change Log, Tasks & Acceptance (+1 more)

### Community 37 - "go_pkg_github_com_artemiosu_staffy_internal_platform_id"
Cohesion: 0.05
Nodes (86): limitedAudioBuffer, TestVerifyRuntimeDatabaseRejectsRLSBypass(), TestVerifyRuntimeDatabaseReportsConnectionFailure(), verifyRuntimeDatabase(), executePurge(), main(), purge(), run() (+78 more)

### Community 38 - "app/behavior_test.go"
Cohesion: 0.12
Nodes (22): behaviorRepo, AllowedClaim, BehaviorRuleError, BehaviorCommand, Service, behaviorFixture(), TestBehaviorClaimScopeRejectsDuplicateChannelAndMode(), TestBehaviorEvidenceMetadataNeverGrantsClaim() (+14 more)

### Community 39 - "Context"
Cohesion: 0.20
Nodes (4): mustScope(), Workspace, Context, New()

### Community 42 - "api/voice.go"
Cohesion: 0.21
Nodes (7): VoicePreview, VoicePreviewRequest, lengthSummary(), Service, safePreviewText(), VoiceRepository, fixtureWAV()

### Community 43 - "Principal"
Cohesion: 0.17
Nodes (6): Fixed, New(), VoiceProfile, VoiceCatalog(), Principal, resolverFixture

### Community 45 - "identity.tenants"
Cohesion: 0.67
Nodes (3): identity.tenants, identity.invitations, identity.membership_audit

### Community 46 - "KnowledgeSource"
Cohesion: 0.10
Nodes (25): KnowledgeChunk, go_pkg_crypto_hmac, knowledgeHTTPFake, knowledgeRow, KnowledgeCreateCommand, KnowledgeFixtureCommand, KnowledgeIngestCommand, KnowledgeProcessCommand (+17 more)

### Community 47 - "validate.go"
Cohesion: 0.14
Nodes (19): go_pkg_go_parser, go_pkg_go_token, go_pkg_io_fs, go_pkg_net, go_pkg_path_filepath, net.IP, net.Resolver, net/url.URL (+11 more)

### Community 48 - "time.Time"
Cohesion: 0.06
Nodes (27): failingIDs, fakeRepository, fixedClock, fixedIDs, packClock, time.Time, fixedClock, employeeIDs (+19 more)

### Community 49 - "testing.T"
Cohesion: 0.07
Nodes (66): packIDs, mapping(), TestBehaviorContractIsTenantScopedAndDraftOnly(), TestStaffyContractHasTenantCreationBoundary(), go_pkg_gopkg_in_yaml_v3, github.com/jackc/pgx/v5/pgxpool.Pool, io.Writer, net/url.Values (+58 more)

### Community 51 - "retentionTx"
Cohesion: 0.33
Nodes (3): pgx.Row, github.com/jackc/pgx/v5/pgconn.CommandTag, retentionTx

### Community 54 - "domain/voice.go"
Cohesion: 0.11
Nodes (17): ESpeakNG, PlanStep, Pronunciation, ReadbackPlan, VoiceRuleError, WorkingHours, go_pkg_unicode_utf8, Brief (+9 more)

### Community 56 - "PackManifest"
Cohesion: 0.21
Nodes (13): VariantError, VariantField, PinCommand, PackImpact, PackManifest, PackPin, PackPreview, TestReferencePackVariantsAndImmutableCatalog() (+5 more)

### Community 57 - "UUID"
Cohesion: 0.12
Nodes (13): sync.Mutex, KnowledgeConflictCommand, KnowledgeRetireCommand, VoiceAssessmentCommand, VoiceCommand, VoiceAssessment, VoicePreviewRecord, VoiceVersion (+5 more)

### Community 59 - "spec-1-5-knowledge-sources.md"
Cohesion: 0.20
Nodes (9): Boundaries & Constraints, Code Map, I/O & Edge-Case Matrix, Implementation Notes, Intent, Review Triage Log, Spec Change Log, Tasks & Acceptance (+1 more)

### Community 60 - "employee.knowledge_sources"
Cohesion: 0.39
Nodes (7): employee.knowledge_audit, employee.knowledge_chunks, employee.knowledge_outbox, employee.knowledge_sources, employee.knowledge_versions, employee, employee.drafts

### Community 62 - "spec-1-6-safe-behavior-configuration.md"
Cohesion: 0.18
Nodes (10): Boundaries & Constraints, Code Map, I/O & Edge-Case Matrix, Implementation Notes, Intent, Review Triage Log, Spec Change Log, Tasks & Acceptance (+2 more)

### Community 63 - "spec-1-7-voice-and-pronunciation.md"
Cohesion: 0.18
Nodes (10): Boundaries & Constraints, Code Map, Design Notes, I/O & Edge-Case Matrix, Implementation Notes, Intent, Review Triage Log, Spec Change Log (+2 more)

### Community 65 - "spec-1-5-safe-source-ingestion.md"
Cohesion: 0.20
Nodes (9): Boundaries & Constraints, Code Map, I/O & Edge-Case Matrix, Implementation Notes, Intent, Review Triage Log, Spec Change Log, Tasks & Acceptance (+1 more)

### Community 66 - "employee.behavior_versions"
Cohesion: 0.50
Nodes (3): employee.behavior_versions, employee, employee.drafts

### Community 67 - "employee.voice_versions"
Cohesion: 0.50
Nodes (3): employee.voice_versions, employee, employee.drafts

### Community 68 - "Draft"
Cohesion: 0.13
Nodes (11): packRepository, fakeEmployee, SaveCommand, Draft, CreateParams, PinParams, SaveParams, pgx.Row (+3 more)

### Community 69 - "actor_id"
Cohesion: 0.67
Nodes (3): minLength, type, actor_id

### Community 70 - "approved_at"
Cohesion: 0.67
Nodes (3): format, type, approved_at

### Community 71 - "role"
Cohesion: 0.67
Nodes (3): role, minLength, type

### Community 72 - "sha256"
Cohesion: 0.67
Nodes (3): sha256, pattern, type

### Community 73 - "tested_at"
Cohesion: 0.67
Nodes (3): tested_at, format, type

### Community 74 - "uri"
Cohesion: 0.67
Nodes (3): uri, minLength, type

### Community 82 - "employee.voice_previews"
Cohesion: 0.50
Nodes (4): employee.voice_versions, employee.voice_previews, employee, voice_previews_employee_idx

## Knowledge Gaps
- **287 isolated node(s):** `$schema`, `$id`, `type`, `additionalProperties`, `required` (+282 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 377 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **18 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `net/http.Request` to `Service`, `testing.T`, `Principal`, `go_pkg_github_com_artemiosu_staffy_internal_platform_id`?**
  _High betweenness centrality (0.001) - this node is a cross-community bridge._
- **Why does `New()` connect `net/http.Request` to `store/membership.go`?**
  _High betweenness centrality (0.001) - this node is a cross-community bridge._
- **Why does `UUID` connect `UUID` to `net/http.Request`?**
  _High betweenness centrality (0.001) - this node is a cross-community bridge._
- **What connects `$schema`, `$id`, `type` to the rest of the system?**
  _287 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `properties` be split into smaller, more focused modules?**
  _Cohesion score 0.05714285714285714 - nodes in this community are weakly interconnected._
- **Should `properties` be split into smaller, more focused modules?**
  _Cohesion score 0.058823529411764705 - nodes in this community are weakly interconnected._
- **Should `properties` be split into smaller, more focused modules?**
  _Cohesion score 0.10526315789473684 - nodes in this community are weakly interconnected._