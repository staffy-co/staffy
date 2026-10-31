package verifier

import (
        "context"
        "net/url"
        "regexp"
        "strconv"
        "strings"

        connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
        connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
)

// AdapterBuild identifies the adapter binary that produced evidence rows.
// A build change invalidates prior evidence as stale (derived at read time).
const AdapterBuild = "synthetic/1.1.0"

// sandboxSecret matches the published fictional sandbox credential format.
var sandboxSecret = regexp.MustCompile(`^sandbox-[a-z0-9][a-z0-9-]{7,63}$`)

// Synthetic performs the deterministic local basic check. It is an explicitly
// fictional provider: results are honest about being synthetic and never claim
// provider capabilities. Secrets and raw responses are redacted by design —
// only predefined reason strings leave this type.
type Synthetic struct{}

// BasicCheck verifies the fictional authentication format and endpoint
// availability rule. It never returns secret material or provider payloads.
func (v Synthetic) BasicCheck(_ context.Context, def connectorDomain.ConnectorDefinition, env connectorDomain.Environment, config map[string]string, secrets map[string]string) connectorDomain.CheckResult {
        result := connectorDomain.CheckResult{Authentication: connectorDomain.CheckFailed, Transport: connectorDomain.CheckFailed}
        if !def.Connectable() {
                result.AuthReason = "Адаптер этого Connector не реализован; подключение недоступно."
                result.TransportReason = "Адаптер этого Connector не реализован; подключение недоступно."
                return result
        }
        authOK := false
        for _, f := range def.SecretSchema() {
                value, ok := secrets[f.Name]
                if !ok {
                        continue
                }
                if sandboxSecret.MatchString(value) {
                        authOK = true
                }
        }
        if authOK {
                result.Authentication = connectorDomain.CheckVerified
        } else {
                result.AuthReason = "Синтетическая проверка аутентификации не прошла: ключ не соответствует формату sandbox-… Используйте тестовый ключ формата sandbox-<8-64 символа>."
        }
        endpoint := ""
        for _, f := range def.ConfigSchema() {
                if f.Type == connectorDomain.FieldURL {
                        if value, ok := config[f.Name]; ok {
                                endpoint = value
                        }
                }
        }
        if transportOK(endpoint, env) {
                result.Transport = connectorDomain.CheckVerified
        } else {
                result.TransportReason = "Синтетическая проверка транспорта не прошла: endpoint должен быть вида https://<имя>." + envSuffix(env) + ". Реальный провайдер не опрашивается."
        }
        return result
}

func envSuffix(env connectorDomain.Environment) string {
        if env == connectorDomain.EnvTest {
                return "test.example"
        }
        return "sandbox.example"
}

// ProbeResult is the redacted outcome of the deterministic read-only probe.
// Outcome uses the domain evidence vocabulary (verified/failed/unknown) and
// DeliveryLevel distinguishes acceptance from delivered business data.
// RedactedRequest/RedactedResult are fixed server-side strings: the probe
// never echoes secret material, query parameters or raw provider payloads.
type ProbeResult = connectorAPI.ProbeResult

// Probe exercises one declared read-only (non-effecting) capability on the
// synthetic contour. Deterministic scenarios are selected by the endpoint
// host prefix: "timeout." answers nothing before the deadline (result
// unknown), "fail." is an honest probe failure; anything else answers at the
// "delivered" receipt level. A context deadline that already expired maps to
// unknown as well: a possibly-effective request is never retried blind.
// The secrets parameter (Story 7-4) is deliberately ignored: the synthetic
// formats never need key material — the telephony behavior is unchanged.
func (v Synthetic) Probe(ctx context.Context, def connectorDomain.ConnectorDefinition, dec connectorDomain.CapabilityDeclaration, env connectorDomain.Environment, config map[string]string, _ map[string]string) ProbeResult {
        // Fail-closed guard: only completed, outbound, probe-allowed query
        // capabilities may ever be exercised here.
        if !dec.ProbeVerifiable() {
                return ProbeResult{
                        Outcome:         connectorDomain.EvidenceFailed,
                        DeliveryLevel:   string(connectorDomain.DeliveryUnknown),
                        RedactedRequest: "probe:" + dec.ID + " — отказ до выполнения",
                        RedactedResult:  "Probe не разрешён для этой capability: проверка мутирующих функций запрещена архитектурой (Action Gateway, AG-4).",
                }
        }
        endpoint := ""
        for _, f := range def.ConfigSchema() {
                if f.Type == connectorDomain.FieldURL {
                        if value, ok := config[f.Name]; ok {
                                endpoint = value
                        }
                }
        }
        host := ""
        if u, err := url.Parse(strings.TrimSpace(endpoint)); err == nil {
                host = u.Hostname()
        }
        request := "probe:read-only " + dec.ID + " → " + host + " (" + string(env) + "), адаптер " + AdapterBuild
        if ctx.Err() != nil || strings.HasPrefix(host, "timeout.") {
                return ProbeResult{
                        Outcome:         connectorDomain.EvidenceUnknown,
                        DeliveryLevel:   string(connectorDomain.DeliveryUnknown),
                        RedactedRequest: request,
                        RedactedResult:  "Probe не завершился до deadline " + deadlineText(dec) + ": результат неопределён (после возможного эффекта). Повтор вслепую запрещён; владелец reconciliation: " + reconciliationOwner(dec) + ". Безопасный следующий шаг: проверьте состояние вручную и запустите проверку повторно осознанно.",
                }
        }
        if strings.HasPrefix(host, "fail.") {
                return ProbeResult{
                        Outcome:         connectorDomain.EvidenceFailed,
                        DeliveryLevel:   string(connectorDomain.DeliveryUnknown),
                        RedactedRequest: request,
                        RedactedResult:  "Синтетический контур отклонил probe (маркер fail. в имени endpoint): возможность не подтверждена для этого аккаунта.",
                }
        }
        return ProbeResult{
                Outcome:         connectorDomain.EvidenceVerified,
                DeliveryLevel:   string(connectorDomain.DeliveryDelivered),
                RedactedRequest: request,
                RedactedResult:  "Синтетический read-only ответ получен: транк активен; уровень доставки delivered (принятие запроса — не бизнес-результат; committed здесь не заявляется).",
        }
}

// Build identifies the adapter build stored with every evidence row.
func (v Synthetic) Build() string { return AdapterBuild }

func deadlineText(dec connectorDomain.CapabilityDeclaration) string {
        if dec.TimeoutMS <= 0 {
                return "не задан"
        }
        return strconv.Itoa(dec.TimeoutMS) + " мс"
}

func reconciliationOwner(dec connectorDomain.CapabilityDeclaration) string {
        if dec.ReconciliationOwner != "" {
                return dec.ReconciliationOwner
        }
        return "Technical Lead"
}

func transportOK(endpoint string, env connectorDomain.Environment) bool {
        if endpoint == "" {
                return false
        }
        u, err := url.Parse(endpoint)
        if err != nil {
                return false
        }
        host := u.Hostname()
        suffix := envSuffix(env)
        // The published rule is "<name>.<suffix>": the suffix must be preceded by
        // a separating dot, so lookalike hosts ("notsandbox.example") never pass.
        if len(host) < len(suffix)+2 {
                return false
        }
        return host[len(host)-len(suffix):] == suffix && host[len(host)-len(suffix)-1] == '.'
}
