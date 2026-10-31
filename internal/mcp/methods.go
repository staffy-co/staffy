package mcp

// The method machines of the surface (Story 7.3 step-02): the revision
// negotiation of server/discover, the gated tool dictionary of
// tools/list and the validated call path of tools/call. The exposure
// gate is the intersection of the facts: the artifact is implemented,
// the determinism class is the platform dictionary word (the
// write-impossibility holds AT this layer — the layer never trusts the
// source's dictionary), the runtime adapter is registered by the
// composition root, and the declared dependency is live for the tenant
// of the session. The call path re-runs the gate (the honest TOCTOU),
// validates the arguments against the CLOSED input schema with the
// field names in every refusal (the model self-corrects — SEP-1303),
// executes the adapter under the declared context budget with the tenant
// of the SESSION (never an argument), and validates the output
// fail-closed — an undeclared key never travels to the client (AC-2).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/tools"
)

// The dictionary words of the honest exposure refusals (the audit
// reasons and the tool execution error words).
const (
	refusalNotImplemented = "контракт не реализован (not_implemented): контракт видим в реестре, но вызов невозможен до адаптера"
	refusalWrongClass     = "класс детерминизма контракта вне платформенного словаря: поверхность экспонирует только deterministic_read"
	refusalNoAdapter      = "адаптер не зарегистрирован в composition root: регистрация контракта не означает доступность вызова"
)

// The declared execution budget of one tool call (the fixture world
// scripts a shorter budget through WithCallBudget).
const defaultCallBudget = 10 * time.Second

// DiscoverResult is the answer of the revision negotiation: the exact
// supported list, the FACTUAL capabilities (tools only, no change
// notifications — the exposure depends on the live proofs) and the
// server identity (the OpenAPI info.version mirror).
type DiscoverResult struct {
	ProtocolVersions []string      `json:"protocolVersions"`
	Capabilities     Capabilities  `json:"capabilities"`
	ServerInfo       ServerInfoDTO `json:"serverInfo"`
}

// Capabilities states the factual surface: nothing the platform does
// not carry is ever claimed (resources, prompts, sampling and
// elicitation stay unclaimed).
type Capabilities struct {
	Tools ToolsCapability `json:"tools"`
}

// ToolsCapability carries the list semantics of the tools facet.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ServerInfoDTO is the server identity of the discover answer.
type ServerInfoDTO struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ListToolsResult is the gated tool dictionary of the tenant session:
// the honest empty array (never null) when the gate closes everything.
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolResult is the answer of one tool call: the pinned contract
// identity in the content, the typed output in the structured content,
// and the honest error flag of the tool execution refusals (the
// self-correcting words — SEP-1303).
type CallToolResult struct {
	Content           []ContentBlock `json:"content"`
	StructuredContent map[string]any `json:"structuredContent,omitempty"`
	IsError           bool           `json:"isError,omitempty"`
}

// ContentBlock is one content entry of the call answer.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// serverDiscover answers the revision negotiation: the exact supported
// list (the single pinned revision), the factual capabilities and the
// server identity.
func (s *Service) serverDiscover(ctx context.Context, envelope *Envelope) (Outcome, error) {
	if refusal := emptyParamsRefusal(envelope); refusal != nil {
		return *refusal, nil
	}
	result := DiscoverResult{
		ProtocolVersions: SupportedProtocolVersions(),
		Capabilities:     Capabilities{Tools: ToolsCapability{ListChanged: false}},
		ServerInfo:       ServerInfoDTO{Name: ServerName, Version: ServerVersion},
	}
	response := NewResult(envelope.ID, result)
	return Outcome{Status: httpStatusOK, Body: &response}, nil
}

// toolsList answers the gated dictionary: the newest version of every
// contract that passes the exposure facts for the tenant of the session.
// The honest empty array is the AG-4 state (no live adapters), never a
// null and never a defect.
func (s *Service) toolsList(ctx context.Context, envelope *Envelope, actor identityDomain.Principal, tenantID id.UUID) (Outcome, error) {
	if refusal := emptyParamsRefusal(envelope); refusal != nil {
		return *refusal, nil
	}
	exposed := make([]Tool, 0)
	for _, contract := range newestContracts(s.source) {
		live, _, err := s.exposureVerdict(ctx, contract, actor, tenantID)
		if err != nil {
			return Outcome{}, err
		}
		if live {
			exposed = append(exposed, ProjectTool(contract))
		}
	}
	response := NewResult(envelope.ID, ListToolsResult{Tools: exposed})
	return Outcome{Status: httpStatusOK, Body: &response}, nil
}

// callParams is the closed params form of the call: the tool name and
// the arguments object (the strict decode refuses the unknown keys).
type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// toolsCall runs the validated call path of one pinned contract: the
// strict params decode, the Mcp-Name routing coherence, the pin read of
// the newest version through the catalog, the re-run of the exposure
// gate, the closed-schema validation of the arguments, the budgeted
// execution with the tenant of the session, and the fail-closed
// validation of the output. Every refusal is the tool execution error
// with the honest words; the audit line records the verdict.
func (s *Service) toolsCall(ctx context.Context, envelope *Envelope, actor identityDomain.Principal, tenantID id.UUID, claims HeaderClaims) (Outcome, error) {
	var params callParams
	if len(envelope.Params) > 0 {
		if err := strictUnmarshal(envelope.Params, &params); err != nil {
			response := NewError(envelope.ID, CodeInvalidParams, "Параметры вызова: закрытая форма {name, arguments}.", nil)
			return Outcome{Status: httpStatusOK, Body: &response}, nil
		}
	}
	if params.Name == "" {
		response := NewError(envelope.ID, CodeInvalidParams, "Параметр name обязателен.", nil)
		return Outcome{Status: httpStatusOK, Body: &response}, nil
	}
	// The routing coherence of the call: the Mcp-Name header carries the
	// tool name and must agree with the params (SEP-2243).
	if claims.Name == "" {
		refusal := invalidRequestResponse(envelope.ID, "Заголовок Mcp-Name обязателен для tools/call.")
		return Outcome{Status: httpStatusBadRequest, Body: &refusal}, nil
	}
	if claims.Name != params.Name {
		refusal := invalidRequestResponse(envelope.ID, fmt.Sprintf("Заголовок Mcp-Name (%s) расходится с именем инструмента (%s).", claims.Name, params.Name))
		return Outcome{Status: httpStatusBadRequest, Body: &refusal}, nil
	}
	contract, ok := s.newestContract(params.Name)
	if !ok {
		return s.toolRefusal(ctx, tenantID, actor, envelope, "", fmt.Sprintf("инструмент %s не существует в реестре", params.Name)), nil
	}
	pin := contract.ID + "@" + contract.Version
	exposed, reason, err := s.exposureVerdict(ctx, contract, actor, tenantID)
	if err != nil {
		return Outcome{}, err
	}
	if !exposed {
		return s.toolRefusal(ctx, tenantID, actor, envelope, pin, reason), nil
	}
	arguments, decodeErr := decodeCallArguments(params.Arguments)
	if decodeErr != nil {
		response := NewError(envelope.ID, CodeInvalidParams, decodeErr.Error(), nil)
		return Outcome{Status: httpStatusOK, Body: &response}, nil
	}
	values, refusals := validateArguments(contract.InputSchema, arguments)
	if len(refusals) > 0 {
		return s.toolRefusal(ctx, tenantID, actor, envelope, pin, strings.Join(refusals, "; ")), nil
	}
	adapter, _ := s.adapters.Adapter(contract.ID, contract.Version)
	callCtx, cancel := context.WithTimeout(ctx, s.callBudget)
	defer cancel()
	output, adapterErr := adapter(callCtx, tenantID, values)
	if adapterErr != nil {
		reason := "сбой адаптера: " + adapterErr.Error()
		if errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			reason = "бюджет выполнения исчерпан: вызов превысил декларированный таймаут"
		}
		return s.toolRefusal(ctx, tenantID, actor, envelope, pin, reason), nil
	}
	if outputRefusals := validateOutput(contract.OutputSchema, output); len(outputRefusals) > 0 {
		// The fail-closed output: the internal material of a misbehaving
		// adapter NEVER travels to the client (AC-2) — the refusal names
		// the schema fact, not the values.
		return s.toolRefusal(ctx, tenantID, actor, envelope, pin, strings.Join(outputRefusals, "; ")+"; выход не переслан"), nil
	}
	s.audit(ctx, tenantID, actor, pin, "ok", "выполнен")
	result := CallToolResult{
		Content:           []ContentBlock{{Type: "text", Text: pin}},
		StructuredContent: output,
	}
	response := NewResult(envelope.ID, result)
	return Outcome{Status: httpStatusOK, Body: &response}, nil
}

// toolRefusal answers the honest tool execution error: the refusal words
// in the content, the error flag, no structured content — and the audit
// line with the verdict and the reason.
func (s *Service) toolRefusal(ctx context.Context, tenantID id.UUID, actor identityDomain.Principal, envelope *Envelope, pin, reason string) Outcome {
	s.audit(ctx, tenantID, actor, pinOrName(pin, envelope), "refused", reason)
	result := CallToolResult{
		Content: []ContentBlock{{Type: "text", Text: reason}},
		IsError: true,
	}
	response := NewResult(envelope.ID, result)
	return Outcome{Status: httpStatusOK, Body: &response}
}

// pinOrName keeps the audit line honest for the unknown-tool refusal
// (no pin exists — the requested name travels instead).
func pinOrName(pin string, envelope *Envelope) string {
	if pin != "" {
		return pin
	}
	return envelope.Method + ":" + string(envelope.ID)
}

// exposureVerdict answers the exposure facts of one contract for the
// tenant of the session: the artifact implementation, the platform
// determinism dictionary (checked AT this layer — the layer never
// trusts the source's own words), the runtime adapter registration and
// the live dependency. The error return is the honest infrastructure
// failure of the dependency read (the gate never guesses).
func (s *Service) exposureVerdict(ctx context.Context, contract tools.ToolContract, actor identityDomain.Principal, tenantID id.UUID) (bool, string, error) {
	if contract.DeterminismClass != "deterministic_read" {
		return false, refusalWrongClass, nil
	}
	if contract.AdapterStatus != "implemented" {
		return false, refusalNotImplemented, nil
	}
	if _, ok := s.adapters.Adapter(contract.ID, contract.Version); !ok {
		return false, refusalNoAdapter, nil
	}
	if contract.Dependency == nil {
		return true, "", nil
	}
	live, word, err := s.gate.Live(ctx, actor, tenantID, contract.Dependency.ConnectorType)
	if err != nil {
		return false, "", err
	}
	if !live {
		return false, dependencyRefusal(contract.Dependency.ConnectorType, word), nil
	}
	return true, "", nil
}

// dependencyRefusal speaks the honest gate words of a closed dependency.
func dependencyRefusal(connectorType, word string) string {
	return fmt.Sprintf("зависимость не жива: подключение коннектора типа %s — состояние %s", connectorType, word)
}

// newestContract pins the newest version of one id through the catalog
// (the honest semver-core comparison — never the index position).
func (s *Service) newestContract(toolID string) (tools.ToolContract, bool) {
	for _, contract := range newestContracts(s.source) {
		if contract.ID == toolID {
			return contract, true
		}
	}
	return tools.ToolContract{}, false
}

// newestContracts folds the source list onto the newest version of
// every id (the registry grows by appends; the insertion order is not a
// version statement).
func newestContracts(source ToolSource) []tools.ToolContract {
	newest := map[string]tools.ToolContract{}
	order := make([]string, 0)
	for _, contract := range source.List() {
		current, seen := newest[contract.ID]
		if !seen {
			newest[contract.ID] = contract
			order = append(order, contract.ID)
			continue
		}
		if semverCoreGreater(contract.Version, current.Version) {
			newest[contract.ID] = contract
		}
	}
	out := make([]tools.ToolContract, 0, len(order))
	for _, contractID := range order {
		out = append(out, newest[contractID])
	}
	return out
}

// semverCoreGreater reports whether version a is strictly newer than b
// in the semantic core order (both words are the validated semver-core
// of the registry; a malformed word of a fixture source compares as not
// newer — the honest refusal of a broken pin).
func semverCoreGreater(a, b string) bool {
	amajor, aminor, apatch, errA := parseSemverCore(a)
	bmajor, bminor, bpatch, errB := parseSemverCore(b)
	if errA != nil || errB != nil {
		return false
	}
	if amajor != bmajor {
		return amajor > bmajor
	}
	if aminor != bminor {
		return aminor > bminor
	}
	return apatch > bpatch
}

// parseSemverCore parses the numeric dotted triple X.Y.Z.
func parseSemverCore(version string) (int, int, int, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("не semver-core: %s", version)
	}
	numbers := make([]int, 3)
	for i, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return 0, 0, 0, err
		}
		numbers[i] = number
	}
	return numbers[0], numbers[1], numbers[2], nil
}

// emptyParamsRefusal answers the invalid-params refusal of the methods
// that carry no params (discover and list): the params member, when
// present, must be the empty object.
func emptyParamsRefusal(envelope *Envelope) *Outcome {
	if len(envelope.Params) == 0 {
		return nil
	}
	var empty struct{}
	if err := strictUnmarshal(envelope.Params, &empty); err != nil {
		response := NewError(envelope.ID, CodeInvalidParams, "Метод не принимает параметров.", nil)
		return &Outcome{Status: httpStatusOK, Body: &response}
	}
	return nil
}
