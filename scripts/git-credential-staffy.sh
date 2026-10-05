#!/bin/sh
# Secret-free credential helper для GitHub.
# Токен НИКОГДА не хранится на диске: приходит только из env STAFFY_GH_TOKEN
# (владелец выдаёт его в чате на каждую сессию).
# Подключение (делает scripts/bootstrap.sh):
#   git config credential.https://github.com.helper <абсолютный путь к этому скрипту>
echo "username=x-access-token"
echo "password=${STAFFY_GH_TOKEN:-}"
