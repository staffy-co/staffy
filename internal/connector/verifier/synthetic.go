package verifier

import (
        "context"
        "net/url"
        "regexp"

        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
)

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
        if len(host) < len(suffix)+1 {
                return false
        }
        return host[len(host)-len(suffix):] == suffix && host != suffix
}
