package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Story 4.4: the fitment evidence domain (the manager's catalog research as
// the append-only recorded material + the read-time state ladder over the
// recorded evidences with the digest pins and the honest words)
// ---------------------------------------------------------------------------
//
// The honesty core of the slice: the fitment evidence is RECORDED MATERIAL,
// never a derivation (the difference from the part request of 4.3 — the
// manager's catalog research is an event of the world: the source, the
// source record identifier, the source version and the time obtained carry
// the traceability to the Owning System answer by the 2.7 discipline). The
// producer is the operator-manager: NO automatic catalog request is ever
// performed or claimed (the parts_catalog_abcp route is not admitted by the
// 4.1 registry — OQ-5/AG-4 open; the manager path honestly executes the
// OQ-5 safe state). The state is the READ-TIME ladder over the append-only
// records: no records → unknown (the absence of a proof is never converted
// into either a negative or a positive fitment — AC-10); the record age
// over the pack TTL → stale; the pins differ from the CURRENT digests of
// the vehicle profile (4.2) or the part request (4.3) → stale (AC-11 — a
// new version of ANY input expires EVERY dependent evidence); the found
// records of the current records of the different sources contradict →
// conflicted (the human receives BOTH proofs — AC-9); a found record with
// several catalog vehicle ids → ambiguous (the most probable modification
// is never selected — AC-5); the one unambiguous agreeing found → proven.
// The position is called fitting ONLY in the proven state; the exact
// number match is NOT a fitment proof (the constant word of 4.3 — reused);
// LLM similarity, RAG and the memory of a past conversation are NOT a
// proof (FR-52). The manager's decision is scoped to ONE case with the
// pins of the exact input versions (AC-8) — never a universal rule for the
// other cars or the tenant. Nothing is silently fixed: the guard refuses
// with the honest words of the dictionary (WorkRuleError — the transport
// maps it onto the 400 problem).

// ---------------------------------------------------------------------------
// The dictionaries (the pack material words, mirrored by the narrow port —
// the domain never imports the port)
// ---------------------------------------------------------------------------

// The link type words of the auto-parts@5.0.0 dictionary (the enum mirror
// of the port answer — the words are pinned by the pack, the domain renders
// the honest words of the recorded one).
const (
	FitmentLinkOriginal = "original"
	FitmentLinkCross    = "cross"
)

// The catalog source words of the auto-parts@5.0.0 dictionary.
const (
	FitmentSourceAbcpCatalog   = "abcp_catalog"
	FitmentSourceLaximoCatalog = "laximo_catalog"
)

// The fitment state words of the read-time ladder (the enum mirror for the
// panels and the contracts — the state is a DERIVATION, never a stored
// word).
const (
	FitmentStatusProven     = "proven"
	FitmentStatusAmbiguous  = "ambiguous"
	FitmentStatusConflicted = "conflicted"
	FitmentStatusUnknown    = "unknown"
	FitmentStatusStale      = "stale"
)

// FitmentResearchWord is the manager task reason word of the fitment
// research recommendation — the PUBLIC dictionary constant of 2.13 (the
// Story 4.4 extension, the SQL mirror 00021; the step-02 switch onto the
// public constant follows the 4.3 precedent of the part boundary words).
const FitmentResearchWord = HandoffReasonFitmentResearch

// The honest state words (the text, never only color).
const (
	// FitmentUnknownWord is the honest word of the absent proof (AC-10:
	// the absence is never converted into either a negative or a positive
	// fitment).
	FitmentUnknownWord = "применимость не проверена — доказательства нет; отсутствие доказательства не является ни подтверждением, ни отрицанием"

	// FitmentUnknownResearchedWord is the honest word of the researched
	// not-found record: the source was researched, the proof was not found
	// — NEVER «не подходит» (AC-10).
	FitmentUnknownResearchedWord = "источник исследован, доказательство не найдено"

	// FitmentStaleAgeWord is the honest word of the over-TTL record (AC-10:
	// the age exceeds the allowed one).
	FitmentStaleAgeWord = "доказательство устарело: возраст превышает допустимый"

	// FitmentStaleProfileWord is the honest word of the profile pin mismatch
	// (AC-11: a new version of the vehicle profile expires the dependent
	// evidence).
	FitmentStaleProfileWord = "доказательство относилось к прежней версии профиля автомобиля"

	// FitmentStalePartRequestWord is the honest word of the request pin
	// mismatch (AC-11: a new version of the part request expires the
	// dependent evidence).
	FitmentStalePartRequestWord = "доказательство относилось к прежней версии запроса"

	// FitmentStaleRecheckWord is the honest re-check word of every stale
	// state (AC-11: the repeated catalog check happens before the next
	// offer — the contour of 4.5).
	FitmentStaleRecheckWord = "перед новым предложением выполняется повторная каталогическая проверка"

	// FitmentConflictedSourcesWord is the honest word of the found-vs-not
	// contradiction of the current records of the different sources (AC-9:
	// the human receives BOTH proofs).
	FitmentConflictedSourcesWord = "источники противоречат друг другу: позиция не отмечается подходящей — оба доказательства переданы человеку"

	// FitmentConflictedVehiclesWord is the honest word of the different
	// catalog vehicle ids of the current found records (AC-9/AC-12).
	FitmentConflictedVehiclesWord = "источники указывают разные идентификаторы автомобиля каталога: позиция не отмечается подходящей — оба доказательства переданы человеку"

	// FitmentAmbiguousWord is the honest word of the several catalog
	// vehicle ids (AC-5: the most probable modification is never selected).
	FitmentAmbiguousWord = "в каталоге несколько модификаций автомобиля: наиболее вероятная не выбирается — уточните недостающий признак либо передайте человеку"

	// FitmentProvenWord is the honest word of the one unambiguous agreeing
	// found record.
	FitmentProvenWord = "подтверждённо подходит"

	// FitmentCrossChainWord is the honest word of the rendered cross chain
	// (AC-4: the chain is shown WHOLE — from the applicable original
	// position to the analog).
	FitmentCrossChainWord = "cross-цепочка показана целиком: от применимой оригинальной позиции до аналога — глубина и источники ограничены Domain Policy пака"

	// FitmentPriceNote is the honest price word of the candidate positions
	// (AC-12: the price, the availability and the term are never fixed
	// obligations — the actual offers are the contour of 4.5).
	FitmentPriceNote = "цена, наличие и срок — не зафиксированные обязательства: актуальные предложения и сравнение — контур 4-5"

	// FitmentScopeWord is the honest scope word of the manager's decision
	// (AC-8: the record is scoped to ONE case with the pins of the exact
	// input versions — never a universal rule).
	FitmentScopeWord = "решение применимо в пределах одного обращения: запись привязана к элементу работы кейса с пинами точных версий входов — правила для других автомобилей не создаётся"
)

// FitmentStatusWords is the honest word of every state word (the panel and
// the contract mirror the dictionary; the drift is the red contract test).
var FitmentStatusWords = map[string]string{
	FitmentStatusProven:     FitmentProvenWord,
	FitmentStatusAmbiguous:  FitmentAmbiguousWord,
	FitmentStatusUnknown:    FitmentUnknownWord,
	FitmentStatusConflicted: FitmentConflictedSourcesWord,
	FitmentStatusStale:      FitmentStaleAgeWord,
}

// ---------------------------------------------------------------------------
// The policy domain form (the pack material through the app mapping)
// ---------------------------------------------------------------------------

// FitmentPolicyDomain is the domain form of the fitment evidence policy of
// the pinned pack (the app maps the narrow port answer onto it — the
// domain never imports the port; the words are pinned by the pack).
type FitmentPolicyDomain struct {
	LinkTypes      []string `json:"link_types"`
	MaxCrossDepth  int      `json:"max_cross_depth"`
	AllowedSources []string `json:"allowed_sources"`
	MaxAgeSeconds  int      `json:"max_age_seconds"`
}

// linkTypeAllowed answers whether the link type word belongs to the pack
// dictionary.
func (p FitmentPolicyDomain) linkTypeAllowed(linkType string) bool {
	return inDictionary(p.LinkTypes, linkType)
}

// sourceAllowed answers whether the source word belongs to the pack
// dictionary.
func (p FitmentPolicyDomain) sourceAllowed(source string) bool {
	return inDictionary(p.AllowedSources, source)
}

// ---------------------------------------------------------------------------
// The evidence draft (the write command material) and the guard
// ---------------------------------------------------------------------------

// FitmentEvidenceDraft is the material of ONE fitment evidence record the
// manager records (the write command of the app — the producer is the
// operator-manager; the automatic catalog request is never performed). The
// empty CatalogVehicleIDs is the honest not-found record: the source was
// researched, the catalog vehicle id was not found (AC-2 — the absence is
// the honest material, never «неприменимо»).
type FitmentEvidenceDraft struct {
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
}

// GuardFitmentEvidence refuses the dishonest record BEFORE any SQL with
// the honest dictionary words (the WorkRuleError family — the transport
// maps it onto the 400 problem; NOTHING is silently fixed). The guard
// walks the pack dictionaries first (the link types, the sources — the
// Domain Policy of the pack, never the store code), then the chain shape
// (the original carries ONE node; the cross chain carries the whole
// original→analog path within the pack depth), then the found-record
// material (the catalog node and the article are mandatory when the
// catalog vehicle ids exist; the not-found record honestly carries the
// source material only) and the traceability + the pins (the source
// record identifier, the source version, the observed time and BOTH
// digest pins are always mandatory — the 2.7 discipline and AC-1/AC-11).
func GuardFitmentEvidence(policy FitmentPolicyDomain, draft FitmentEvidenceDraft) error {
	if strings.TrimSpace(draft.PositionKey) == "" {
		return WorkRuleError{Rule: "fitment_position_key_required", Reason: "ключ позиции обязателен: запись доказательства привязывается к позиции материала кейса"}
	}
	if !policy.linkTypeAllowed(draft.LinkType) {
		return WorkRuleError{Rule: "fitment_link_type_out_of_dictionary", Reason: "тип связи вне словаря пака: разрешены " + joinQuoted(policy.LinkTypes) + " — Domain Policy пака"}
	}
	if !policy.sourceAllowed(draft.SourceSystem) {
		return WorkRuleError{Rule: "fitment_source_out_of_dictionary", Reason: "источник вне словаря пака: разрешены " + joinQuoted(policy.AllowedSources) + " — Domain Policy пака"}
	}
	// The canonical position key: the normalized rendering of the article
	// (the 4.3 number discipline — the upper case, the dropped spaces, the
	// unified separators); the caller normalizes BEFORE the command, the
	// guard refuses the non-canonical key honestly (never a silent fix).
	if NormalizePartNumber(draft.PositionKey).Normalized != draft.PositionKey {
		return WorkRuleError{Rule: "fitment_position_key_not_canonical", Reason: "канонизируйте ключ позиции: нормализованное представление артикула без пробелов, в верхнем регистре, с унифицированными разделителями"}
	}
	switch draft.LinkType {
	case FitmentLinkOriginal:
		if len(draft.Chain) != 1 {
			return WorkRuleError{Rule: "fitment_original_chain_shape", Reason: "original-доказательство несёт один узел цепочки — оригинальную применимую позицию"}
		}
	case FitmentLinkCross:
		if len(draft.Chain) < 2 {
			return WorkRuleError{Rule: "fitment_cross_chain_too_short", Reason: "cross-цепочка обязана содержать оригинальную позицию и аналог — минимум два узла"}
		}
		if depth := len(draft.Chain) - 1; depth > policy.MaxCrossDepth {
			return WorkRuleError{Rule: "fitment_cross_chain_too_long", Reason: fmt.Sprintf("слишком длинная cross-цепочка: глубина %d превышает разрешённые %d — Domain Policy пака", depth, policy.MaxCrossDepth)}
		}
	}
	if len(draft.CatalogVehicleIDs) > 0 {
		if strings.TrimSpace(draft.CatalogNode) == "" {
			return WorkRuleError{Rule: "fitment_catalog_node_required", Reason: "каталогический узел или группа детали обязателен при найденном доказательстве"}
		}
		if strings.TrimSpace(draft.Article) == "" {
			return WorkRuleError{Rule: "fitment_article_required", Reason: "артикул/OEM обязателен при найденном доказательстве"}
		}
	}
	if strings.TrimSpace(draft.SourceRecordID) == "" {
		return WorkRuleError{Rule: "fitment_source_record_id_required", Reason: "идентификатор записи источника обязателен — трассируемость до ответа Owning System"}
	}
	if strings.TrimSpace(draft.SourceVersion) == "" {
		return WorkRuleError{Rule: "fitment_source_version_required", Reason: "версия источника обязательна — трассируемость до ответа Owning System"}
	}
	if draft.SourceObservedAt.IsZero() {
		return WorkRuleError{Rule: "fitment_source_observed_at_required", Reason: "время получения материала источника обязательно — трассируемость до ответа Owning System"}
	}
	if draft.ProfileDigestPin == "" {
		return WorkRuleError{Rule: "fitment_profile_digest_pin_required", Reason: "пин ProfileDigest обязателен: доказательство обязано пиннить точную версию профиля автомобиля"}
	}
	if draft.PartRequestDigestPin == "" {
		return WorkRuleError{Rule: "fitment_part_request_digest_pin_required", Reason: "пин PartRequestDigest обязателен: доказательство обязано пиннить точную версию запроса детали"}
	}
	return nil
}

// ---------------------------------------------------------------------------
// The read-time state ladder (AC-5/AC-9/AC-10/AC-11)
// ---------------------------------------------------------------------------

// FitmentEvidenceInput is ONE append-only evidence record handed to the
// derivation (the store material — the record of the manager's catalog
// research with the traceability fields and the digest pins).
type FitmentEvidenceInput struct {
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
}

// FitmentProfileAttribute is ONE clarifiable profile attribute of the
// derivation input (the optional identification attributes of 4.2 — the
// label comes from the pack schema, the domain never knows the key words).
type FitmentProfileAttribute struct {
	Key       string
	Label     string
	Confirmed bool
}

// FitmentDerivationInput is the whole material of the read-time fitment
// derivation of one case: the pack policy, the positions with their
// append-only evidence records (ordered by the evidence number), the
// CURRENT digests of the vehicle profile (4.2) and the part request (4.3),
// the clock now, the safety-node flag (the pack markers over the part kind
// — the 4.3 discipline, the app input) and the clarifiable profile
// attributes (the ambiguous question material — AC-5).
type FitmentDerivationInput struct {
	Policy                       FitmentPolicyDomain
	Positions                    []FitmentPositionInput
	ProfileDigest                string
	PartRequestDigest            string
	Now                          time.Time
	SafetyNode                   bool
	ClarifiableProfileAttributes []FitmentProfileAttribute
}

// FitmentPositionInput is ONE position with its append-only evidence
// records (the store reads them ordered by the evidence number).
type FitmentPositionInput struct {
	PositionKey string
	Evidences   []FitmentEvidenceInput
}

// FitmentEvidenceView is the rendered material of ONE evidence record (the
// panel shows the source, the source record identifier, the version and
// the times as the details — the traceability without leaving the case,
// AC-14) with the per-record stale words.
type FitmentEvidenceView struct {
	EvidenceNo           int64     `json:"evidence_no"`
	SourceSystem         string    `json:"source_system"`
	SourceRecordID       string    `json:"source_record_id"`
	SourceVersion        string    `json:"source_version"`
	SourceObservedAt     time.Time `json:"source_observed_at"`
	ReceivedAt           time.Time `json:"received_at"`
	LinkType             string    `json:"link_type"`
	Chain                []string  `json:"chain"`
	CatalogVehicleIDs    []string  `json:"catalog_vehicle_ids"`
	CatalogNode          string    `json:"catalog_node"`
	Article              string    `json:"article"`
	ProfileDigestPin     string    `json:"profile_digest_pin"`
	PartRequestDigestPin string    `json:"part_request_digest_pin"`
	StaleWords           []string  `json:"stale_words"`
	Found                bool      `json:"found"`
}

// FitmentPositionView is the read-time fitment answer of ONE position: the
// state word with the honest words, the evidence records material (BOTH
// records under the contradiction — AC-9), the FitmentDigest (the exact
// version of the proof — the pin of the contours 4.5..4.9, present ONLY in
// the proven state), the ambiguous questions (the minimal question of the
// missing profile attribute — AC-5) and the handoff recommendation words
// (the 2.13 dictionary).
type FitmentPositionView struct {
	PositionKey   string                `json:"position_key"`
	Status        string                `json:"status"`
	StatusWord    string                `json:"status_word"`
	Words         []string              `json:"words"`
	Evidences     []FitmentEvidenceView `json:"evidences"`
	FitmentDigest string                `json:"fitment_digest,omitempty"`
	Questions     map[string]string     `json:"questions"`
	HandoffWords  []string              `json:"handoff_words"`
}

// FitmentView is the whole read-time fitment answer of the case: the
// positions with their states, the case-level unknown word when NO
// evidence exists at all (the honest «применимость не проверена» + the
// research recommendation), the constant word of the exact number match
// (the 4.3 constant — the match is never a fitment proof), the price note
// (AC-12) and the scope word (AC-8).
type FitmentView struct {
	Positions        []FitmentPositionView `json:"positions"`
	CaseUnknownWord  string                `json:"case_unknown_word,omitempty"`
	CaseHandoffWords []string              `json:"case_handoff_words,omitempty"`
	NumberMatchWord  string                `json:"number_match_word"`
	PriceNote        string                `json:"price_note"`
	ScopeWord        string                `json:"scope_word"`
}

// DeriveFitment derives the read-time fitment states of the case over the
// append-only evidence records (READ-TIME: zero writes, idempotent — the
// JSON of the same world is byte-stable). The ladder per position: the
// CURRENT record of every source is the one with the maximal evidence
// number (the supersede is the NEW number, never an update); NO records at
// all → the case-level unknown; every current record is expired (the age
// over the pack TTL or the pin mismatch with the CURRENT digests) → stale
// with the honest words + the re-check word; the current found records of
// the different sources contradict (the found against the not-found or the
// different catalog vehicle ids) → conflicted with BOTH records; a found
// record with several catalog vehicle ids → ambiguous with the minimal
// question of the missing profile attribute (the confirmed attributes are
// never re-asked); the one unambiguous agreeing found → proven with the
// FitmentDigest (the exact version of the proof — the pin of 4.5..4.9).
func DeriveFitment(input FitmentDerivationInput) FitmentView {
	view := FitmentView{
		Positions:       []FitmentPositionView{},
		NumberMatchWord: PartNumberMatchIsNotFitmentText,
		PriceNote:       FitmentPriceNote,
		ScopeWord:       FitmentScopeWord,
	}
	if len(input.Positions) == 0 {
		view.CaseUnknownWord = FitmentUnknownWord
		view.CaseHandoffWords = []string{FitmentResearchWord}
		return view
	}
	for _, position := range input.Positions {
		view.Positions = append(view.Positions, deriveFitmentPosition(input, position))
	}
	return view
}

// deriveFitmentPosition derives the state of ONE position.
func deriveFitmentPosition(input FitmentDerivationInput, position FitmentPositionInput) FitmentPositionView {
	out := FitmentPositionView{
		PositionKey:  position.PositionKey,
		Questions:    map[string]string{},
		HandoffWords: []string{},
		Evidences:    []FitmentEvidenceView{},
	}
	if len(position.Evidences) == 0 {
		out.Status = FitmentStatusUnknown
		out.StatusWord = FitmentUnknownWord
		out.HandoffWords = append(out.HandoffWords, FitmentResearchWord)
		return out
	}
	// The current record of every source: the maximal evidence number of
	// that source (the supersede discipline — the NEW number, never an
	// update; the store reads them ordered, the derivation re-elects the
	// current ones honestly).
	current := map[string]FitmentEvidenceInput{}
	for _, evidence := range position.Evidences {
		existing, ok := current[evidence.SourceSystem]
		if !ok || evidence.EvidenceNo > existing.EvidenceNo {
			current[evidence.SourceSystem] = evidence
		}
	}
	currentList := make([]FitmentEvidenceInput, 0, len(current))
	for _, evidence := range current {
		currentList = append(currentList, evidence)
	}
	sort.SliceStable(currentList, func(i, j int) bool {
		return currentList[i].SourceSystem < currentList[j].SourceSystem
	})
	// The stale words of every current record (the age and the pins — the
	// honest words of the record material).
	expired := 0
	for _, evidence := range currentList {
		evidenceView := fitmentEvidenceView(evidence)
		if isExpiredEvidence(input, evidence) {
			expired++
			if age := input.Now.Sub(evidence.ReceivedAt); age > time.Duration(input.Policy.MaxAgeSeconds)*time.Second {
				evidenceView.StaleWords = append(evidenceView.StaleWords, FitmentStaleAgeWord)
			}
			if evidence.ProfileDigestPin != input.ProfileDigest {
				evidenceView.StaleWords = append(evidenceView.StaleWords, FitmentStaleProfileWord)
			}
			if evidence.PartRequestDigestPin != input.PartRequestDigest {
				evidenceView.StaleWords = append(evidenceView.StaleWords, FitmentStalePartRequestWord)
			}
		}
		out.Evidences = append(out.Evidences, evidenceView)
	}
	// The whole ladder over the CURRENT records.
	if expired == len(currentList) {
		out.Status = FitmentStatusStale
		out.StatusWord = FitmentStaleAgeWord
		out.Words = append(out.Words, FitmentStaleRecheckWord)
		return out
	}
	// The current LIVE records (the expired ones are honestly shown above
	// — their material stays in the view; the ladder derives over the live
	// ones only).
	live := make([]FitmentEvidenceInput, 0, len(currentList))
	for _, evidence := range currentList {
		if !isExpiredEvidence(input, evidence) {
			live = append(live, evidence)
		}
	}
	// Classify the live records: the not-found and the found ones with
	// their catalog vehicle ids.
	type liveClass struct {
		notFound  bool
		vehicleID string
		many      bool
	}
	classes := make([]liveClass, 0, len(live))
	for _, evidence := range live {
		switch {
		case len(evidence.CatalogVehicleIDs) == 0:
			classes = append(classes, liveClass{notFound: true})
		case len(evidence.CatalogVehicleIDs) > 1:
			classes = append(classes, liveClass{many: true})
		default:
			classes = append(classes, liveClass{vehicleID: evidence.CatalogVehicleIDs[0]})
		}
	}
	notFoundCount, foundCount, manyCount := 0, 0, 0
	ids := map[string]struct{}{}
	for _, class := range classes {
		switch {
		case class.notFound:
			notFoundCount++
		case class.many:
			foundCount++
			manyCount++
		default:
			foundCount++
			ids[class.vehicleID] = struct{}{}
		}
	}
	switch {
	case notFoundCount > 0 && foundCount > 0:
		out.Status = FitmentStatusConflicted
		out.StatusWord = FitmentConflictedSourcesWord
		out.HandoffWords = append(out.HandoffWords, HandoffReasonUnresolvedContradiction)
	case foundCount > 0 && len(ids) > 1:
		out.Status = FitmentStatusConflicted
		out.StatusWord = FitmentConflictedVehiclesWord
		out.HandoffWords = append(out.HandoffWords, HandoffReasonUnresolvedContradiction)
	case manyCount > 0:
		out.Status = FitmentStatusAmbiguous
		out.StatusWord = FitmentAmbiguousWord
		fitmentAmbiguousQuestion(input, &out)
	case foundCount > 0:
		out.Status = FitmentStatusProven
		out.StatusWord = FitmentProvenWord
		out.FitmentDigest = fitmentDigestOf(live[0])
		if input.SafetyNode {
			out.Words = append(out.Words, PartRequestSafetyNoteText)
		}
	default:
		// Every live record is a not-found one: the honest researched
		// unknown — NEVER «неприменимо» (AC-10); the position stays out of
		// the confirmed-fitting candidates.
		out.Status = FitmentStatusUnknown
		out.StatusWord = FitmentUnknownResearchedWord
	}
	return out
}

// isExpiredEvidence answers whether ONE evidence record is expired by the
// age or the digest pins (the AC-11 read-time expiry).
func isExpiredEvidence(input FitmentDerivationInput, evidence FitmentEvidenceInput) bool {
	if age := input.Now.Sub(evidence.ReceivedAt); age > time.Duration(input.Policy.MaxAgeSeconds)*time.Second {
		return true
	}
	if evidence.ProfileDigestPin != input.ProfileDigest {
		return true
	}
	if evidence.PartRequestDigestPin != input.PartRequestDigest {
		return true
	}
	return false
}

// fitmentEvidenceView renders the material view of ONE evidence record.
func fitmentEvidenceView(evidence FitmentEvidenceInput) FitmentEvidenceView {
	return FitmentEvidenceView{
		EvidenceNo:           evidence.EvidenceNo,
		SourceSystem:         evidence.SourceSystem,
		SourceRecordID:       evidence.SourceRecordID,
		SourceVersion:        evidence.SourceVersion,
		SourceObservedAt:     evidence.SourceObservedAt,
		ReceivedAt:           evidence.ReceivedAt,
		LinkType:             evidence.LinkType,
		Chain:                append([]string(nil), evidence.Chain...),
		CatalogVehicleIDs:    append([]string(nil), evidence.CatalogVehicleIDs...),
		CatalogNode:          evidence.CatalogNode,
		Article:              evidence.Article,
		ProfileDigestPin:     evidence.ProfileDigestPin,
		PartRequestDigestPin: evidence.PartRequestDigestPin,
		StaleWords:           []string{},
		Found:                len(evidence.CatalogVehicleIDs) > 0,
	}
}

// fitmentAmbiguousQuestion renders the minimal question of the FIRST
// not-confirmed clarifiable profile attribute (AC-5: the confirmed
// attributes are never re-asked; the question asks the MISSING one). No
// clarifiable attribute left — the honest handoff recommendation of the
// manager research (the 2.13 dictionary word of this story).
func fitmentAmbiguousQuestion(input FitmentDerivationInput, out *FitmentPositionView) {
	for _, attribute := range input.ClarifiableProfileAttributes {
		if attribute.Confirmed {
			continue
		}
		out.Questions[attribute.Key] = fmt.Sprintf("уточните: %s? — в каталоге несколько модификаций автомобиля, наиболее вероятная не выбирается", strings.ToLower(attribute.Label))
		return
	}
	out.HandoffWords = append(out.HandoffWords, FitmentResearchWord)
}

// fitmentDigestOf renders the SHA-256 of the canonical JSON of the proof
// record — {position_key, link_type, chain, source, source_version, pins}
// with the sorted keys (the byte-stable canonical form of «the exact
// version of the proof», the pin of the contours 4.5..4.9).
func fitmentDigestOf(evidence FitmentEvidenceInput) string {
	canonical := map[string]any{
		"position_key":            evidence.PositionKey,
		"link_type":               evidence.LinkType,
		"chain":                   evidence.Chain,
		"source":                  evidence.SourceSystem,
		"source_version":          evidence.SourceVersion,
		"profile_digest_pin":      evidence.ProfileDigestPin,
		"part_request_digest_pin": evidence.PartRequestDigestPin,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		// The shape is fixed (the strings and the string slices) — the
		// marshal cannot fail; the honest fallback keeps the derivation
		// total.
		encoded = []byte("{}")
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// FitmentDigestOf exposes the canonical digest of ONE evidence record (the
// app persists it with the record — the store material of the exact proof
// version; the derivation recomputes it read-time, byte-stable).
func FitmentDigestOf(evidence FitmentEvidenceInput) string {
	return fitmentDigestOf(evidence)
}
