#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""step-01 surgical patch 3/4: internal/cases/port/port.go
PackTaxonomy +PackFitmentSchema + тип FitmentPolicySpec (простые типы порта)."""
import io

P = 'internal/cases/port/port.go'
s = io.open(P, encoding='utf-8').read()

# 1) Метод в интерфейсе PackTaxonomy (после PackPartRequestSchema, перед закрывающей скобкой)
old_iface = '''\tPackPartRequestSchema(packID, packVersion string) (PartRequestSchema, error)
}'''
new_iface = '''\tPackPartRequestSchema(packID, packVersion string) (PartRequestSchema, error)
\t// PackFitmentSchema resolves the fitment evidence policy of the pinned
\t// pack (Story 4.4) — the link types, the maximal cross chain depth, the
\t// allowed catalog sources and the maximal evidence age. The same
\t// immutability argument as the journeys and the identification schema:
\t// the catalog resolves the same policy for the same (id, version), no
\t// TOCTOU exists; a pack without a policy is the honest absent answer,
\t// the caller degrades the panel honestly.
\tPackFitmentSchema(packID, packVersion string) (FitmentPolicySpec, error)
}'''
assert old_iface in s, 'port.go: interface anchor not found'
s = s.replace(old_iface, new_iface, 1)

# 2) Тип FitmentPolicySpec после PartRequestSchema (перед CommandLookup)
old_type = '''// CommandLookup is the idempotency answer of one command key BEFORE any'''
new_type = '''// FitmentPolicySpec is the port answer of the pack fitment policy (Story
// 4.4): the simple structural type of the narrow port — the allowed link
// types, the maximal cross chain depth, the allowed catalog sources and
// the maximal evidence age (the TTL of the proof freshness — the Domain
// Policy of the pack, never the code of one store).
type FitmentPolicySpec struct {
\tLinkTypes      []string
\tMaxCrossDepth  int
\tAllowedSources []string
\tMaxAgeSeconds  int
}

// CommandLookup is the idempotency answer of one command key BEFORE any'''
assert old_type in s, 'port.go: type anchor not found'
s = s.replace(old_type, new_type, 1)

io.open(P, 'w', encoding='utf-8').write(s)
print('port.go: +PackFitmentSchema method, +FitmentPolicySpec OK')
