package httptransport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
)

func releaseFormatDay(t time.Time) string { return t.Format("02.01.2006") }
func releaseFormatTimestamp(t time.Time) string {
	return t.Format("02.01.2006 15:04 UTC")
}

// releaseStatusText keeps the manifest status textual — a comparison never
// depends on color alone.
func releaseStatusText(status string) string {
	switch status {
	case employeeDomain.ManifestStatusCandidate:
		return "candidate (все компоненты на месте)"
	default:
		return "incomplete (есть явные gap-ы)"
	}
}

func releaseRefStatusText(status string) string {
	switch status {
	case employeeDomain.RefPresent:
		return "настроен"
	case employeeDomain.RefNotApplicable:
		return "неприменим для этого сотрудника"
	default:
		return "не настроен — честное отсутствие"
	}
}

// releaseMissingDependencies lists the honest dependencies that keep every
// sealed manifest of this slice a verifiable candidate, never an admitted
// release: the runtime components, the Operating Profile and the gate
// evidence, each with its owner.
func releaseMissingDependencies() []dependencyRow {
	return []dependencyRow{
		{Dependency: "Runtime-компоненты (маршруты модели и медиа, сборка промптов, схемы инструментов, пакет политик, eval, сборка runtime, огибающая автономности)", Owner: "AI Runtime (Epic 2) · ответственный: Technical Lead"},
		{Dependency: "Утверждённый Operating Profile", Owner: "Story 1-13 (утверждение operating profile и правил измерения)"},
		{Dependency: "Обязательные gate evidence и admission", Owner: "владельцы GATE-REGISTRY · допуск: Story 1-14"},
	}
}

// prepareReleaseView converts the service view into the template payload.
func prepareReleaseView(view employeeAPI.ReleaseView) releasePageView {
	data := releasePageView{
		CanManage: view.CanManage,
		ScopeText: releaseScopeText(view.ViewScope),
		Versions:  []versionRow{},
	}
	if view.Draft != nil {
		data.Draft = prepareDraftView(*view.Draft)
	}
	sealedBy := map[int64]employeeAPI.ManifestSummary{}
	for _, summary := range view.Manifests {
		sealedBy[summary.ConfigVersion] = summary
	}
	for index, version := range view.Versions {
		row := versionRow{
			Version:     version.Version,
			Description: version.Description,
			ActorID:     version.ActorID.String(),
			CreatedAt:   releaseFormatTimestamp(version.CreatedAt),
		}
		// The changed components compare this version's refs with the
		// previous one (text, never color alone).
		var previous *employeeDomain.ConfigRefs
		if index+1 < len(view.Versions) {
			refs := view.Versions[index+1].Refs
			previous = &refs
		}
		row.Changed = releaseChangedComponents(version.Refs, previous)
		if summary, ok := sealedBy[version.Version]; ok {
			row.Sealed = true
			row.Status = summary.Status
			row.StatusText = releaseStatusText(summary.Status)
			row.Digest = summary.Digest
		} else {
			row.StatusText = "не запечатан"
		}
		data.Versions = append(data.Versions, row)
	}
	if view.Latest != nil {
		manifest := prepareManifestView(*view.Latest)
		data.Latest = &manifest
	}
	return data
}

func releaseScopeText(scope string) string {
	if scope == employeeAPI.ReleaseScopeManage {
		return "configuration — просмотр и управление"
	}
	return "review/audit — только просмотр, управление недоступно"
}

// releaseChangedComponents lists the components whose references differ
// between a version and its predecessor — comparing the exact versions,
// not only presence: a bumped brief, a moved behavior version, an added
// knowledge source or a changed voice version all count. The first version
// lists every component it pins (there is nothing to compare with).
func releaseChangedComponents(refs employeeDomain.ConfigRefs, previous *employeeDomain.ConfigRefs) string {
	changed := []string{}
	record := func(name, current, prev string) {
		if previous == nil || current != prev {
			changed = append(changed, name)
		}
	}
	prevBrief, prevBehavior, prevVoice, prevKnowledge, prevPack := "", "", "", "", ""
	if previous != nil {
		prevBrief = fmt.Sprintf("%s/%d", previous.Brief.Status, previous.Brief.DraftVersion)
		prevBehavior = fmt.Sprintf("%s/%d", previous.Behavior.Status, previous.Behavior.Version)
		prevVoice = fmt.Sprintf("%s/%d", previous.Voice.Status, previous.Voice.Version)
		prevKnowledge = knowledgeFingerprint(previous.Knowledge)
		prevPack = packFingerprint(previous.Pack)
	}
	record("Brief", fmt.Sprintf("%s/%d", refs.Brief.Status, refs.Brief.DraftVersion), prevBrief)
	record("Domain Pack", packFingerprint(refs.Pack), prevPack)
	record("Поведение", fmt.Sprintf("%s/%d", refs.Behavior.Status, refs.Behavior.Version), prevBehavior)
	record("Знания", knowledgeFingerprint(refs.Knowledge), prevKnowledge)
	record("Голос", fmt.Sprintf("%s/%d", refs.Voice.Status, refs.Voice.Version), prevVoice)
	return strings.Join(changed, ", ")
}

// packFingerprint pins the exact pack reference including variants count.
func packFingerprint(pack employeeDomain.PackRef) string {
	if pack.Status != employeeDomain.RefPresent {
		return pack.Status
	}
	return pack.ID + "@" + pack.Version + "/" + strconv.Itoa(len(pack.Variants))
}

// knowledgeFingerprint pins the exact source list with versions.
func knowledgeFingerprint(knowledge employeeDomain.KnowledgeRef) string {
	if knowledge.Status != employeeDomain.RefPresent {
		return knowledge.Status
	}
	parts := make([]string, 0, len(knowledge.Sources))
	for _, source := range knowledge.Sources {
		parts = append(parts, source.SourceID.String()+"@"+strconv.FormatInt(source.Version, 10))
	}
	return strings.Join(parts, ";")
}

func prepareDraftView(draft employeeDomain.ReleaseDraft) *draftPageView {
	data := &draftPageView{
		Revision:  draft.Revision,
		UpdatedAt: releaseFormatTimestamp(draft.UpdatedAt),
	}
	data.Components = append(data.Components,
		componentRow{
			Name: "Employee Brief", Status: draft.Refs.Brief.Status, StatusText: releaseRefStatusText(draft.Refs.Brief.Status),
			Value: fmt.Sprintf("черновик v%d", draft.Refs.Brief.DraftVersion), Note: draft.Refs.Brief.Note,
		},
		componentRow{
			Name: "Domain Pack", Status: draft.Refs.Pack.Status, StatusText: releaseRefStatusText(draft.Refs.Pack.Status),
			Value: releasePackValue(draft.Refs.Pack), Note: draft.Refs.Pack.Note,
		},
		componentRow{
			Name: "Поведение", Status: draft.Refs.Behavior.Status, StatusText: releaseRefStatusText(draft.Refs.Behavior.Status),
			Value: releaseVersionValue(draft.Refs.Behavior.Status, draft.Refs.Behavior.Version), Note: draft.Refs.Behavior.Note,
		},
		componentRow{
			Name: "Знания", Status: draft.Refs.Knowledge.Status, StatusText: releaseRefStatusText(draft.Refs.Knowledge.Status),
			Value: releaseKnowledgeValue(draft.Refs.Knowledge), Note: draft.Refs.Knowledge.Note,
		},
		componentRow{
			Name: "Голос", Status: draft.Refs.Voice.Status, StatusText: releaseRefStatusText(draft.Refs.Voice.Status),
			Value: releaseVersionValue(draft.Refs.Voice.Status, draft.Refs.Voice.Version), Note: draft.Refs.Voice.Note,
		},
	)
	if draft.RestoredFrom != nil {
		data.RestoredFrom = &restoredRow{
			Version:   draft.RestoredFrom.Version,
			VersionID: draft.RestoredFrom.VersionID.String(),
			CreatedAt: releaseFormatTimestamp(draft.RestoredFrom.CreatedAt),
		}
	}
	return data
}

func releasePackValue(pack employeeDomain.PackRef) string {
	if pack.Status != employeeDomain.RefPresent {
		return "—"
	}
	value := pack.ID + " " + pack.Version
	if len(pack.Variants) > 0 {
		value += fmt.Sprintf(" · варианты: %d", len(pack.Variants))
	}
	return value
}

func releaseVersionValue(status string, version int64) string {
	if status != employeeDomain.RefPresent {
		return "—"
	}
	return fmt.Sprintf("v%d", version)
}

func releaseKnowledgeValue(knowledge employeeDomain.KnowledgeRef) string {
	if knowledge.Status != employeeDomain.RefPresent || len(knowledge.Sources) == 0 {
		return "—"
	}
	value := fmt.Sprintf("%d источник(ов)", len(knowledge.Sources))
	for _, source := range knowledge.Sources {
		value += fmt.Sprintf("; %s v%d", source.SourceID, source.Version)
	}
	return value
}

func prepareManifestView(view employeeAPI.ManifestView) manifestPageView {
	data := manifestPageView{
		ConfigVersion:    view.ConfigVersion,
		Status:           view.Status,
		StatusText:       releaseStatusText(view.Status),
		Digest:           view.Digest,
		SchemaVersion:    view.SchemaVersion,
		SchemaText:       fmt.Sprintf("schema_version %d", view.SchemaVersion),
		ActorID:          view.ActorID.String(),
		SealedAt:         releaseFormatTimestamp(view.SealedAt),
		QuarantineKind:   view.Quarantine,
		QuarantineReason: view.QuarantineReason,
		Raw:              view.Raw,
		Dependencies:     releaseMissingDependencies(),
	}
	if view.Quarantine != "" {
		data.Quarantine = true
		data.QuarantineKindText = releaseQuarantineText(view.Quarantine)
	}
	if view.Content != nil {
		data.Connections = prepareConnectionRows(view.Content.Connections)
		data.Runtime = prepareRuntimeRows(view.Content.Runtime)
		data.ConfigDescription = view.Content.Config.Description
	}
	return data
}

func releaseQuarantineText(kind string) string {
	switch kind {
	case employeeDomain.QuarantineSchemaVersion:
		return "quarantine: версия схемы не поддерживается"
	case employeeDomain.QuarantineUnknownField:
		return "quarantine: неизвестное поле или нечитаемый payload"
	case employeeDomain.QuarantineDigestMismatch:
		return "quarantine: нарушена целостность (digest)"
	default:
		return "quarantine"
	}
}

func prepareConnectionRows(connections []employeeDomain.ConnectionFact) []connectionRow {
	rows := make([]connectionRow, 0, len(connections))
	for _, conn := range connections {
		row := connectionRow{
			ConnectionID:    conn.ConnectionID.String(),
			ConnectorID:     conn.ConnectorID,
			ContractVersion: conn.ContractVersion,
			AdapterBuild:    conn.AdapterBuild,
			Environment:     conn.Environment,
			Status:          conn.Status,
			Capabilities:    make([]capabilityRow, 0, len(conn.Capabilities)),
		}
		for _, capability := range conn.Capabilities {
			capRow := capabilityRow{
				Capability: capability.Capability,
				State:      capability.State,
				StateText:  releaseCapabilityStateText(capability.State),
				Reason:     capability.Reason,
				Owner:      capability.Owner,
			}
			if capability.Gap {
				capRow.GapText = "gap: capability не подтверждена для этого подключения"
			}
			if capability.Evidence != nil {
				capRow.EvidenceID = capability.Evidence.EvidenceID.String()
				capRow.Outcome = capability.Evidence.Outcome
				capRow.CheckedAt = releaseFormatTimestamp(capability.Evidence.CheckedAt)
				capRow.ExpiresAt = releaseFormatDay(capability.Evidence.ExpiresAt)
			}
			row.Capabilities = append(row.Capabilities, capRow)
		}
		rows = append(rows, row)
	}
	return rows
}

func releaseCapabilityStateText(state string) string {
	switch state {
	case "verified":
		return "подтверждена для этого аккаунта"
	case "failed":
		return "проверка не пройдена"
	case "stale":
		return "доказательство устарело"
	case "unsupported":
		return "контракт не декларирует семантику"
	case "unknown":
		return "результат неопределён"
	case "verifying":
		return "проверка выполняется"
	default:
		return "не проверена"
	}
}

func prepareRuntimeRows(runtime []employeeDomain.RuntimeComponentState) []runtimeRow {
	dictionary := employeeDomain.RuntimeDictionary()
	titles := map[string]string{}
	for _, spec := range dictionary {
		titles[spec.Component] = spec.Title
	}
	rows := make([]runtimeRow, 0, len(runtime))
	for _, component := range runtime {
		presentText := "отсутствует — явный gap"
		if component.Present {
			presentText = "присутствует: " + component.Ref
		}
		rows = append(rows, runtimeRow{
			Component: component.Component, Title: titles[component.Component],
			Present: component.Present, PresentText: presentText,
			Owner: component.Owner, Responsible: component.Responsible, Note: component.Note,
		})
	}
	return rows
}

// releasePageView is the whole release page payload.
type releasePageView struct {
	CanManage bool
	ScopeText string
	Draft     *draftPageView
	Versions  []versionRow
	Latest    *manifestPageView
}

type draftPageView struct {
	Revision     int64
	Components   []componentRow
	RestoredFrom *restoredRow
	UpdatedAt    string
}

type componentRow struct {
	Name       string
	Status     string
	StatusText string
	Value      string
	Note       string
}

type restoredRow struct {
	Version   int64
	VersionID string
	CreatedAt string
}

type versionRow struct {
	Version     int64
	Description string
	ActorID     string
	CreatedAt   string
	Sealed      bool
	Status      string
	StatusText  string
	Digest      string
	Changed     string
}

type manifestPageView struct {
	ConfigVersion      int64
	ConfigDescription  string
	Status             string
	StatusText         string
	Digest             string
	SchemaVersion      int
	SchemaText         string
	ActorID            string
	SealedAt           string
	Quarantine         bool
	QuarantineKind     string
	QuarantineKindText string
	QuarantineReason   string
	Raw                string
	Connections        []connectionRow
	Runtime            []runtimeRow
	Dependencies       []dependencyRow
}

type connectionRow struct {
	ConnectionID    string
	ConnectorID     string
	ContractVersion string
	AdapterBuild    string
	Environment     string
	Status          string
	Capabilities    []capabilityRow
}

type capabilityRow struct {
	Capability string
	State      string
	StateText  string
	Reason     string
	GapText    string
	EvidenceID string
	Outcome    string
	CheckedAt  string
	ExpiresAt  string
	Owner      string
}

type runtimeRow struct {
	Component   string
	Title       string
	Present     bool
	PresentText string
	Owner       string
	Responsible string
	Note        string
}

type dependencyRow struct {
	Dependency string
	Owner      string
}
