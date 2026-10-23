#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""step-01 surgical patch 4/4: internal/employee/app/route_facts.go
+фасад PackFitmentSchema (сотрудник-сервис структурно удовлетворяет порт)."""
import io

P = 'internal/employee/app/route_facts.go'
s = io.open(P, encoding='utf-8').read()

# Вставка фасада ПОСЛЕ метода PackPartRequestSchema (конец файла/метода —
# якорь: закрывающая скобка метода + конец файла; найдём хвост метода по
# последним строкам PackPartRequestSchema)
anchor = '''\tschema.SafetyMarkers = append([]string(nil), manifest.PartRequests.SafetyMarkers...)
\tschema.PartSearchJourney = manifest.PartRequests.PartSearchJourney
\treturn schema, nil
}'''
assert anchor in s, 'route_facts.go: facade anchor not found'

facade = anchor + '''

// PackFitmentSchema resolves the fitment evidence policy of one Domain
// Pack version (Story 4.4 — the link types, the maximal cross chain
// depth, the allowed catalog sources and the maximal evidence age). The
// same immutability guarantees as PackJourneys; a pack without a policy
// is the honest absent answer (the caller degrades the panel honestly,
// never a hardcoded policy); an unknown pack version is an honest error.
func (s *Service) PackFitmentSchema(packID, packVersion string) (casesPort.FitmentPolicySpec, error) {
\tif packID == "" || packVersion == "" {
\t\treturn casesPort.FitmentPolicySpec{}, fmt.Errorf("пак не закреплён: политика применимости неизвестна")
\t}
\tmanifest, ok := packs.Get(packID, packVersion)
\tif !ok {
\t\treturn casesPort.FitmentPolicySpec{}, fmt.Errorf("закреплённый пак %s@%s не читается из каталога: манифест отсутствует", packID, packVersion)
\t}
\tpolicy := casesPort.FitmentPolicySpec{LinkTypes: []string{}, AllowedSources: []string{}}
\tif manifest.Fitment == nil {
\t\treturn policy, nil
\t}
\tpolicy.LinkTypes = append([]string(nil), manifest.Fitment.LinkTypes...)
\tpolicy.MaxCrossDepth = manifest.Fitment.MaxCrossDepth
\tpolicy.AllowedSources = append([]string(nil), manifest.Fitment.AllowedSources...)
\tpolicy.MaxAgeSeconds = manifest.Fitment.MaxAgeSeconds
\treturn policy, nil
}'''
s = s.replace(anchor, facade, 1)

io.open(P, 'w', encoding='utf-8').write(s)
print('route_facts.go: +PackFitmentSchema facade OK')
