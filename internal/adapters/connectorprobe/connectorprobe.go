package connectorprobe

// The composition router of the connector verifier (Story 7-4): the single
// api.Verifier of the application routes by the RequiredAccountType marker
// of the catalog definition — the external MCP class ("external-mcp") runs
// its live probes through the client machine internal/mcpclient with the
// MAC-checked secret of the verify command, every other class keeps the
// synthetic verifier of 1-8 (the telephony behavior is byte-identical).
// The Build identity is the ROUTER's own constant: every evidence row the
// application saves now carries connectorprobe/1.0.0, so the telephony
// evidence of the earlier builds honestly reads as stale after the deploy —
// the operators re-probe (the 1-8 "build change invalidates evidence"
// semantics, fixed in the DEVLOG of the step).
//
// The router never imports the connector module and the connector module
// never knows the router: the composition root adapts (the
// capabilityresolver/toolssource precedents of 3-1/7-3).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
	connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
	"github.com/artemiosu/staffy/internal/mcpclient"
)

// AdapterBuild identifies the router build stored with every evidence row.
// The change from synthetic/1.1.0 to connectorprobe/1.0.0 invalidates the
// prior evidence as stale at read time — the honest derivation never
// carries a success of an older build forward.
const AdapterBuild = "connectorprobe/1.0.0"

// ExternalMCPAccountType is the catalog marker of the external MCP class —
// the routing key of the router (the definition's RequiredAccountType).
const ExternalMCPAccountType = "external-mcp"

// The machine budget of the probe answers — the mirror of the declared
// SizeLimit of the class card (256 КиБ): a foreign answer past the budget
// is refused whole, without truncation and without passthrough.
const probeResponseBudgetBytes = 256 << 10

// externalSecretField is the single declared secret field of the class
// card — the MAC boundary of the slice (the seal covers the full submitted
// map, the card declares exactly one secret).
const externalSecretField = "api_key"

// sandboxSecret matches the published fictional sandbox credential format
// (the BasicCheck branch of the class — the same format discipline as the
// synthetic telephony: no real credentials exist, AG-4).
var sandboxSecret = regexp.MustCompile(`^sandbox-[a-z0-9][a-z0-9-]{7,63}$`)

// Router is the stateless composition verifier: the external MCP branch
// plus the synthetic fallback of the classes without a dedicated adapter.
type Router struct {
	fallback connectorAPI.Verifier
}

// New wires the synthetic fallback (the classes of 1-8 keep their behavior
// byte-identical).
func New(fallback connectorAPI.Verifier) *Router {
	return &Router{fallback: fallback}
}

// The structural pin: the router satisfies the consumer-owned port.
var _ connectorAPI.Verifier = (*Router)(nil)

// routes reports whether the definition belongs to the external MCP class.
func routes(def connectorDomain.ConnectorDefinition) bool {
	return def.RequiredAccountType == ExternalMCPAccountType
}

// BasicCheck runs the format checks of the class WITHOUT any network
// round: the sandbox key format of the submitted secret and the URL form
// of the endpoint. The connect path (sealing) stays the 1-8 path — an
// invalid credential is never stored.
func (r *Router) BasicCheck(ctx context.Context, def connectorDomain.ConnectorDefinition, env connectorDomain.Environment, config map[string]string, secrets map[string]string) connectorDomain.CheckResult {
	if !routes(def) {
		return r.fallback.BasicCheck(ctx, def, env, config, secrets)
	}
	result := connectorDomain.CheckResult{Authentication: connectorDomain.CheckFailed, Transport: connectorDomain.CheckFailed}
	if !def.Connectable() {
		result.AuthReason = "Адаптер этого Connector не реализован; подключение недоступно."
		result.TransportReason = "Адаптер этого Connector не реализован; подключение недоступно."
		return result
	}
	authOK := false
	if value, ok := secrets[externalSecretField]; ok && sandboxSecret.MatchString(value) {
		authOK = true
	}
	if authOK {
		result.Authentication = connectorDomain.CheckVerified
	} else {
		result.AuthReason = "Проверка аутентификации внешнего MCP-сервера не прошла: ключ не соответствует формату sandbox-… Используйте тестовый ключ формата sandbox-<8-64 символа>."
	}
	endpoint := strings.TrimSpace(config["endpoint"])
	if validEndpoint(endpoint) {
		result.Transport = connectorDomain.CheckVerified
	} else {
		result.TransportReason = "Проверка транспорта внешнего MCP-сервера не прошла: endpoint обязан быть корректным URL (https://…). Реальный сервер на этапе базовой проверки не опрашивается."
	}
	return result
}

// Probe runs the live authenticated rounds of the external MCP class
// through the client machine: query.server-discover (the revision
// negotiation) and query.tools-list (the inventory source). The secrets
// parameter is the MAC-checked key material of the verify command — it
// becomes the Authorization material of the rounds and never travels into
// any persisted word. The fail-closed guard mirrors the synthetic: only
// probe-verifiable capabilities ever reach the wire.
func (r *Router) Probe(ctx context.Context, def connectorDomain.ConnectorDefinition, dec connectorDomain.CapabilityDeclaration, env connectorDomain.Environment, config map[string]string, secrets map[string]string) connectorAPI.ProbeResult {
	if !routes(def) {
		return r.fallback.Probe(ctx, def, dec, env, config, secrets)
	}
	if !dec.ProbeVerifiable() {
		return connectorAPI.ProbeResult{
			Outcome:         connectorDomain.EvidenceFailed,
			DeliveryLevel:   string(connectorDomain.DeliveryUnknown),
			RedactedRequest: "probe:" + dec.ID + " — отказ до выполнения",
			RedactedResult:  "Probe не разрешён для этой capability: проверка мутирующих функций запрещена архитектурой (Action Gateway, AG-4).",
		}
	}
	endpoint := strings.TrimSpace(config["endpoint"])
	request := "probe:read-only " + dec.ID + " → " + endpointHost(endpoint) + " (" + string(env) + "), адаптер " + AdapterBuild
	client := mcpclient.New(http.DefaultTransport, probeResponseBudgetBytes)
	authorization := ""
	if value, ok := secrets[externalSecretField]; ok && value != "" {
		authorization = "Bearer " + value
	}
	switch dec.ID {
	case "query.server-discover":
		return discoverProbe(ctx, client, request, endpoint, authorization, dec)
	case "query.tools-list":
		return listProbe(ctx, client, request, endpoint, authorization, dec)
	default:
		// An unlisted capability of the class never reaches the wire: the
		// declaration and the router agree on the two probe surfaces.
		return connectorAPI.ProbeResult{
			Outcome:         connectorDomain.EvidenceFailed,
			DeliveryLevel:   string(connectorDomain.DeliveryUnknown),
			RedactedRequest: request + " — отказ до выполнения",
			RedactedResult:  "Роутер проб не обслуживает capability " + dec.ID + " этого класса: живые раунды заявлены только для query.server-discover и query.tools-list.",
		}
	}
}

// discoverProbe runs the single revision-negotiation round. The answer
// words are the probe's own fixed strings: the server identity (the
// provenance material) travels, the raw answer never does.
func discoverProbe(ctx context.Context, client *mcpclient.Client, request, endpoint, authorization string, dec connectorDomain.CapabilityDeclaration) connectorAPI.ProbeResult {
	answer, err := client.Discover(ctx, endpoint, authorization)
	if err != nil {
		return probeFailure(request, dec, err)
	}
	return connectorAPI.ProbeResult{
		Outcome:         connectorDomain.EvidenceVerified,
		DeliveryLevel:   string(connectorDomain.DeliveryDelivered),
		RedactedRequest: request,
		RedactedResult: fmt.Sprintf("Раунд server/discover выполнен: сервер %s %s, переговоры ревизии сошлись на %s; уровень доставки delivered (принятие запроса — не бизнес-результат).",
			answer.ServerInfo.Name, answer.ServerInfo.Version, mcpclient.ProtocolVersion),
	}
}

// listProbe runs the inventory rounds (discover + tools/list) and answers
// the honest words with the filtered inventory shape of step-02 (the
// compact JSON inventory lands with the dictionary slice; this step proves
// the rounds and the descriptor count).
func listProbe(ctx context.Context, client *mcpclient.Client, request, endpoint, authorization string, dec connectorDomain.CapabilityDeclaration) connectorAPI.ProbeResult {
	if _, err := client.Discover(ctx, endpoint, authorization); err != nil {
		return probeFailure(request, dec, err)
	}
	answer, err := client.ListTools(ctx, endpoint, authorization)
	if err != nil {
		return probeFailure(request, dec, err)
	}
	return connectorAPI.ProbeResult{
		Outcome:         connectorDomain.EvidenceVerified,
		DeliveryLevel:   string(connectorDomain.DeliveryDelivered),
		RedactedRequest: request,
		RedactedResult: fmt.Sprintf("Раунды server/discover + tools/list выполнены: инвентарь фильтрован до известных полей протокола, read-tools: %d; уровень доставки delivered (принятие запроса — не бизнес-результат).",
			len(answer.Tools)),
	}
}

// probeFailure maps the client refusal onto the evidence vocabulary with
// the named honest reason: the protocol refusals are the failed outcome,
// the deadline exhaustion is the honest unknown of a possibly-effective
// round (never retried blind).
func probeFailure(request string, dec connectorDomain.CapabilityDeclaration, err error) connectorAPI.ProbeResult {
	if errors.Is(err, mcpclient.ErrDeadline) {
		return connectorAPI.ProbeResult{
			Outcome:         connectorDomain.EvidenceUnknown,
			DeliveryLevel:   string(connectorDomain.DeliveryUnknown),
			RedactedRequest: request,
			RedactedResult:  "Раунд не завершился до декларированного бюджета (" + deadlineText(dec) + "): результат неопределён (после возможного эффекта). Повтор вслепую запрещён; владелец reconciliation: " + reconciliationOwner(dec) + ". Безопасный следующий шаг: проверьте состояние внешнего сервера вручную и запустите проверку повторно осознанно.",
		}
	}
	return connectorAPI.ProbeResult{
		Outcome:         connectorDomain.EvidenceFailed,
		DeliveryLevel:   string(connectorDomain.DeliveryUnknown),
		RedactedRequest: request,
		RedactedResult:  "Проба внешнего MCP-сервера не прошла: " + err.Error() + ". Возможность не подтверждена для этого аккаунта.",
	}
}

// Build identifies the adapter build stored with every evidence row: the
// single router constant (a build change invalidates prior evidence).
func (r *Router) Build() string { return AdapterBuild }

// validEndpoint holds the URL form check of the class (the format branch
// of BasicCheck — no network round).
func validEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// endpointHost extracts the host word of the endpoint for the redacted
// request line (never the full URL with any path material).
func endpointHost(raw string) string {
	if u, err := url.Parse(strings.TrimSpace(raw)); err == nil {
		return u.Host
	}
	return "не разобран"
}

func deadlineText(dec connectorDomain.CapabilityDeclaration) string {
	if dec.TimeoutMS <= 0 {
		return "не задан"
	}
	return fmt.Sprintf("%d мс", dec.TimeoutMS)
}

func reconciliationOwner(dec connectorDomain.CapabilityDeclaration) string {
	if dec.ReconciliationOwner != "" {
		return dec.ReconciliationOwner
	}
	return "Technical Lead"
}
