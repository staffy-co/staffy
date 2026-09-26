- source_spec: `spec-1-2-tenant-membership.md`
  summary: Перед публичным развёртыванием проверить, не записывает ли reverse proxy секрет одноразового приглашения из query string.
  evidence: Staffy выставляет no-store и no-referrer, но конфигурация внешнего proxy и его access-log в репозитории отсутствует; проверка фактических логов развёртывания разрешит риск.
