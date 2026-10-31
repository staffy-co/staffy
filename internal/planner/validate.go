package planner

// The strict validation of the Turn Proposal against the Turn Context
// (Story 8-1): the closed enums, the mandatory fields and the
// cross-field rules — the mode ladder (a proposal never lowers the
// conversation's priority mode — which also carries the guarantee
// that no safety flag of the proposal ever removes a safety MODE of
// the context), the union reference of the proposed action, the reply
// budget without truncation, and the read requests against the
// inventory of the context. Every refusal is named; the named cause
// is what the repair round and the journal see.

import (
	"encoding/json"
	"unicode/utf8"
)

// ValidateTurnContext validates a decoded turn context (the field
// rules of the closed form). DecodeTurnContext runs this after the
// strict decode; the export keeps the validation surface honest for
// the composition callers of the future slices.
func ValidateTurnContext(context TurnContext) *RuleError {
	return validateTurnContext(context)
}

// ValidateTurnProposal validates the decoded proposal against the
// decoded context. A nil return means the material is a validated
// proposal of this machine.
func ValidateTurnProposal(proposal TurnProposal, context TurnContext) *RuleError {
	return validateTurnProposal(proposal, context)
}

func validateTurnProposal(proposal TurnProposal, context TurnContext) *RuleError {
	if !isDictionaryWord(proposal.Intent, IntentDictionary) {
		return rule("proposal.intent", "намерение %q вне закрытого словаря намерений среза", proposal.Intent)
	}
	if !isDictionaryWord(proposal.Mode, ModeLadder) {
		return rule("proposal.mode", "режим предложения %q вне закрытого словаря лестницы приоритетов", proposal.Mode)
	}
	if !isDictionaryWord(proposal.Safety, SafetyDictionary) {
		return rule("proposal.safety", "safety-состояние %q вне закрытого словаря safety-состояний", proposal.Safety)
	}
	// The ladder rule: the proposal mode may not be LOWER (less strict)
	// than the context mode. This is also the enforcement that no
	// safety word of the proposal ever lifts the safety regime of the
	// context — the mode stays where the conversation put it.
	if ModeRank(proposal.Mode) > ModeRank(context.Mode) {
		return rule("proposal.mode_ladder", "режим предложения %q понижает режим разговора %q по лестнице приоритетов — понижения не существует", proposal.Mode, context.Mode)
	}
	for _, suggestion := range proposal.FactProposals {
		if suggestion.Key == "" {
			return rule("proposal.fact_provenance", "ключ fact-предложения не может быть пустым")
		}
		if refusal := platformValueRule("proposal.fact_value", suggestion.Key, suggestion.Value); refusal != nil {
			return refusal
		}
		if refusal := provenanceRule("proposal.fact_provenance", suggestion.Key, suggestion.Provenance); refusal != nil {
			return refusal
		}
	}
	for _, conflict := range proposal.Contradictions {
		if conflict.Key == "" {
			return rule("proposal.contradictions", "ключ противоречия не может быть пустым")
		}
		if refusal := platformValueRule("proposal.contradictions", conflict.Key, conflict.Proposed); refusal != nil {
			return refusal
		}
		if refusal := provenanceRule("proposal.contradictions", conflict.Key, conflict.Provenance); refusal != nil {
			return refusal
		}
	}
	for _, request := range proposal.ReadRequests {
		if refusal := validateReadRequest(request, context); refusal != nil {
			return refusal
		}
	}
	for _, question := range proposal.Questions {
		if question.Text == "" {
			return rule("proposal.question_budget", "текст вопроса не может быть пустым")
		}
		if runes := utf8.RuneCountInString(question.Text); runes > ReplyBudgetRunes {
			return rule("proposal.question_budget", "вопрос из %d рун превышает бюджет реплики %d — усечения не существует", runes, ReplyBudgetRunes)
		}
	}
	if !isDictionaryWord(proposal.ProposedAction.Kind, ActionKindDictionary) {
		return rule("proposal.action_kind", "вид предлагаемого действия %q вне закрытого union-словаря", proposal.ProposedAction.Kind)
	}
	if proposal.ProposedAction.Kind == ActionKindNone {
		if proposal.ProposedAction.ContractRef != "" {
			return rule("proposal.action_ref", "вид none не несёт контракт-референса — референс допустим только для commitment | action | handoff")
		}
	} else if !isRef(proposal.ProposedAction.ContractRef) {
		return rule("proposal.action_ref", "вид %q обязан нести контракт-референс вида id@версия — предложение только называет", proposal.ProposedAction.Kind)
	}
	if proposal.Reply.Text == "" {
		return rule("proposal.reply_budget", "текст реплики не может быть пустым — ход обязан нести реплику")
	}
	if runes := utf8.RuneCountInString(proposal.Reply.Text); runes > ReplyBudgetRunes {
		return rule("proposal.reply_budget", "реплика из %d рун превышает платформенный бюджет %d — усечения не существует", runes, ReplyBudgetRunes)
	}
	for _, evidence := range proposal.EvidenceRefs {
		if evidence.Type == "" || evidence.ID == "" {
			return rule("proposal.evidence_refs", "evidence-ссылка обязана нести тип и идентификатор")
		}
	}
	for _, entry := range proposal.MemoryPatch {
		if entry.Key == "" {
			return rule("proposal.memory_patch", "ключ записи патча Case-памяти не может быть пустым")
		}
		if refusal := platformValueRule("proposal.memory_patch", entry.Key, entry.Value); refusal != nil {
			return refusal
		}
		if refusal := provenanceRule("proposal.memory_patch", entry.Key, entry.Provenance); refusal != nil {
			return refusal
		}
	}
	return nil
}

// validateReadRequest validates one read request against the tool
// inventory of the context: the tool must be declared (name@version
// exactly), every argument must be a declared schema field of the
// right platform type, every required field must be present, string
// values must be inside the enum and integers inside the bounds. The
// dictionary discipline of 7-2/7-4 applied to the model's asks.
func validateReadRequest(request ReadRequest, context TurnContext) *RuleError {
	if !isRef(request.Tool) {
		return rule("proposal.read_tool", "инструмент read-запроса %q обязан быть именем@версией", request.Tool)
	}
	var schema map[string]ToolSchemaField
	found := false
	for _, tool := range context.Tools {
		if tool.Name == request.Tool {
			schema = tool.InputSchema
			found = true
			break
		}
	}
	if !found {
		return rule("proposal.read_tool", "инструмент %q отсутствует в инвентаре контекста — ход может читать только заявленное", request.Tool)
	}
	for field := range request.Args {
		if _, declared := schema[field]; !declared {
			return rule("proposal.read_arg", "аргумент %q не объявлен в схеме входа инструмента %q", field, request.Tool)
		}
	}
	for field, fieldSchema := range schema {
		value, present := request.Args[field]
		if !present {
			if !fieldSchema.Optional {
				return rule("proposal.read_arg", "обязательный аргумент %q инструмента %q отсутствует", field, request.Tool)
			}
			continue
		}
		switch fieldSchema.Type {
		case "string":
			text, ok := value.(string)
			if !ok {
				return rule("proposal.read_arg", "аргумент %q инструмента %q обязан быть строкой", field, request.Tool)
			}
			if len(fieldSchema.Values) > 0 && !isDictionaryWord(text, fieldSchema.Values) {
				return rule("proposal.read_arg", "строка аргумента %q инструмента %q вне enum-словаря схемы", field, request.Tool)
			}
		case "integer":
			number, ok := value.(json.Number)
			if !ok {
				return rule("proposal.read_arg", "аргумент %q инструмента %q обязан быть целым числом", field, request.Tool)
			}
			if !isIntegerText(number.String()) {
				return rule("proposal.read_arg", "аргумент %q инструмента %q обязан быть строгим целым литералом, получено %q", field, request.Tool, number.String())
			}
			parsed := mustInt(number)
			if fieldSchema.Min != 0 && parsed < fieldSchema.Min {
				return rule("proposal.read_arg", "целое аргумента %q инструмента %q ниже границы %d", field, request.Tool, fieldSchema.Min)
			}
			if fieldSchema.Max != 0 && parsed > fieldSchema.Max {
				return rule("proposal.read_arg", "целое аргумента %q инструмента %q выше границы %d", field, request.Tool, fieldSchema.Max)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return rule("proposal.read_arg", "аргумент %q инструмента %q обязан быть булевым значением", field, request.Tool)
			}
		default:
			return rule("proposal.read_arg", "тип аргумента %q инструмента %q вне платформенного словаря типизации", field, request.Tool)
		}
	}
	return nil
}

// mustInt parses an already-validated integer literal (the dictionary
// check ran first — the parse cannot fail here).
func mustInt(number json.Number) int {
	parsed, _ := number.Int64()
	return int(parsed)
}
