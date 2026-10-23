#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""step-02 surgical patch 2/3: порт + api.
a) port.go: +FitmentEvidenceCommandMaterial + WorkCommandParams.Fitment + Repository.ReadFitmentEvidencesOfCase + FitmentEvidenceRecord;
b) api.go: WorkOutcome +Story 4.4 material;
c) api/fitment.go (новый файл — пишется отдельно)."""
import io

# --- a) port.go
P = 'internal/cases/port/port.go'
s = io.open(P, encoding='utf-8').read()

# 1) WorkCommandParams +Fitment поле (после HandoffReason, найдём якорь поля)
anchor1 = '''	HandoffReason        string
'''
assert anchor1 in s, 'port.go: WorkCommandParams anchor not found'
s = s.replace(anchor1, anchor1 + '''	// Fitment is the Story 4.4 record material of the
	// record_fitment_evidence command (nil for every other command — the
	// one-structure material of the append-only proof write).
	Fitment               *FitmentEvidenceCommandMaterial
''', 1)

# 2) Типы FitmentEvidenceCommandMaterial + FitmentEvidenceRecord — вставим после CommandLookup
anchor2 = '''// WorkTargetParams is one split target: the app-generated anchor id with
// the type and the title.'''
assert anchor2 in s, 'port.go: types anchor not found'
types = '''// FitmentEvidenceCommandMaterial is the Story 4.4 record material of ONE
// fitment evidence write: the generated evidence identifier, the position
// key (canonical), the found/not-found catalog material (the node, the
// article, the link type, the whole cross chain, the catalog vehicle ids
// — empty is the honest not-found record), the source traceability (the
// system word, the record identifier, the version, the observed time),
// BOTH digest pins (the exact input versions — AC-1/AC-11) and the
// canonical FitmentDigest of the record.
type FitmentEvidenceCommandMaterial struct {
	EvidenceID           id.UUID
	PositionKey          string
	CatalogNode          string
	Article              string
	LinkType             string
	Chain                []string
	CatalogVehicleIDs    []string
	SourceSystem         string
	SourceRecordID       string
	SourceVersion        string
	SourceObservedAt     time.Time
	ProfileDigestPin     string
	PartRequestDigestPin string
	FitmentDigest        string
}

// FitmentEvidenceRecord is ONE stored evidence record of the read surface
// (the store reads them ordered by the evidence number — the read-time
// ladder elects the current record of every source by the number).
type FitmentEvidenceRecord struct {
	WorkItemID           id.UUID
	EvidenceID           id.UUID
	EvidenceNo           int64
	PositionKey          string
	CatalogNode          string
	Article              string
	LinkType             string
	Chain                []string
	CatalogVehicleIDs    []string
	SourceSystem         string
	SourceRecordID       string
	SourceVersion        string
	SourceObservedAt     time.Time
	ReceivedAt           time.Time
	ProfileDigestPin     string
	PartRequestDigestPin string
	FitmentDigest        string
	CreatedAt            time.Time
}

''' + anchor2
s = s.replace(anchor2, types, 1)

# 3) Repository +ReadFitmentEvidencesOfCase (после ExecuteWorkCommand — вставим после его сигнатуры)
anchor3 = '''	ExecuteWorkCommand(ctx context.Context, p WorkCommandParams) error'''
assert anchor3 in s, 'port.go: repository anchor not found'
s = s.replace(anchor3, anchor3 + '''
	// ReadFitmentEvidencesOfCase reads every fitment evidence record of
	// the case (the Story 4.4 read surface — the read-time state ladder
	// input, ordered by the work item, the position, the source and the
	// evidence number).
	ReadFitmentEvidencesOfCase(ctx context.Context, scope tenancy.Context, caseID id.UUID) ([]FitmentEvidenceRecord, error)''', 1)

io.open(P, 'w', encoding='utf-8').write(s)
print('port.go: +material, +record, +repository method')

# --- b) api.go WorkOutcome +fitment поля
P2 = 'internal/cases/api/api.go'
s2 = io.open(P2, encoding='utf-8').read()
anchor4 = '''	HandoffTurnRef     id.UUID `json:"handoff_turn_ref,omitempty"`'''
assert anchor4 in s2, 'api.go: WorkOutcome anchor not found'
s2 = s2.replace(anchor4, '''	// The Story 4.4 material: the fitment evidence record of the write
	// command (the generated identifier, the monotonic number, the
	// position and the canonical digest — «the exact version of the
	// proof» for the contours 4.5..4.9).
	FitmentEvidenceID     id.UUID `json:"fitment_evidence_id,omitempty"`
	FitmentEvidenceNo     int64   `json:"fitment_evidence_no,omitempty"`
	FitmentPositionKey    string  `json:"fitment_position_key,omitempty"`
	FitmentDigest         string  `json:"fitment_digest,omitempty"`
''' + anchor4, 1)
io.open(P2, 'w', encoding='utf-8').write(s2)
print('api.go: WorkOutcome +fitment material')
