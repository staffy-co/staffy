#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""step-01 surgical patch 1/4: internal/employee/domain/pack.go
+FitmentPolicySchema тип + PackManifest.Fitment поле (nil-совместимость паков без схемы)."""
import io

P = 'internal/employee/domain/pack.go'
s = io.open(P, encoding='utf-8').read()

# 1) Поле Fitment в PackManifest (после PartRequests, перед AtLeastOneOf)
old_field = """\tPartRequests      *PartRequestSchema    `json:"part_requests,omitempty"`
\tAtLeastOneOf      []string              `json:"at_least_one_of,omitempty"`"""
new_field = """\tPartRequests      *PartRequestSchema    `json:"part_requests,omitempty"`
\t// Fitment is the fitment evidence policy of Story 4.4: nil for every
\t// pack without one (the 1.0.0..4.0.0 auto-parts and montagio-services
\t// stay nil — the immutability of the published versions is never
\t// violated; the first policy arrives with auto-parts@5.0.0).
\tFitment           *FitmentPolicySchema  `json:"fitment,omitempty"`
\tAtLeastOneOf      []string              `json:"at_least_one_of,omitempty"`"""
assert old_field in s, 'pack.go: field anchor not found'
s = s.replace(old_field, new_field, 1)

# 2) Тип FitmentPolicySchema (после PartRequestSchema, перед VariantField)
old_type = """type VariantField struct {"""
new_type = """// FitmentPolicySchema is the versioned fitment evidence policy of one
// Domain Pack (Story 4.4): the allowed link types (original | cross), the
// maximal cross chain depth, the allowed catalog sources and the maximal
// evidence age (the TTL of the proof freshness). The policy is declarative
// setup metadata — the Domain Policy of the pack, never the code of one
// store; the domain guards and the read-time state ladder derive over it.
type FitmentPolicySchema struct {
\tLinkTypes     []string `json:"link_types"`
\tMaxCrossDepth int      `json:"max_cross_depth"`
\tAllowedSources []string `json:"allowed_sources"`
\tMaxAgeSeconds int      `json:"max_age_seconds"`
}

type VariantField struct {"""
assert old_type in s, 'pack.go: type anchor not found'
s = s.replace(old_type, new_type, 1)

io.open(P, 'w', encoding='utf-8').write(s)
print('pack.go: +FitmentPolicySchema, +Fitment field OK')
