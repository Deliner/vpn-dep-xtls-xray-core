---
name: security-audit
description: Resolve this repository's govulncheck findings or failed security scans while preserving dependency pins and required behavior.
---

# Уязвимость или незавершённое сканирование

Прочитай [общий процесс](../../refactor/SKILL.md) и исходную диагностику.
`just audit` проверяет Go-код и тесты на Linux/macOS × amd64/arm64, отчёты —
в `build/reports/security/`. `just audit-runtime` скачивает закреплённые Linux-движки
для обеих CPU, проверяет SHA/size/ELF и сканирует их Go symbols/dependencies;
отчёты — в `build/reports/security-runtime/`.

При найденной уязвимости открой её GO-ID/advisory: установи затронутую версию,
условия эксплуатации, исправленную версию и путь вызова в нашем коде. Отличай
production-путь от тестового. Предпочитай поддерживаемое обновление модуля/toolchain
с сохранением API и поведения. Измени закреплённую версию, выполни `just deps-tidy`,
проверь diff `go.mod`/`go.sum`, затем `just audit` и `just check`. Для runtime-изменений
нужен `just payload`. Обновляя Go, синхронизируй toolchain manifest и проверенные SHA.

Для runtime-находки проверь `binary_sha256`, Go build info и actual module versions
в отчёте. Binary mode не строит call graph и может сообщать о недостижимом коде.
Если main module имеет `(devel)`, его собственные advisories этим сканированием не
покрыты: отдельно проверь upstream release/security notes для закреплённого engine.
Для fork/replacement-пакета выясни, перенесён ли патч из upstream; одно сходство версий
не доказывает исправление. Локальные replacements дополнительно проверяются по
исходной upstream module/version из build info. Найденный advisory требует review
даже при уже перенесённом патче; отсутствие версии, неполный ответ или ошибка запроса
не дают успешного аудита. Сверь raw query JSON и `upstream_audit_error` в отчёте.
Этот запрос использует внутренний query mode закреплённого scanner; при смене его
протокола обнови проверку полного ответа и её негативные тесты, не отключай проверку.
При обновлении runtime синхронизируй assets/SHA/size,
test manifest и client/server pins, затем выполни `just audit-runtime`, `just check`,
`just payload-setup` и `just payload`. Не меняй неподтверждённый checksum ради загрузки.
Go-сканер не проверяет C-код и libcronet; успешный ELF check не является аудитом их CVE.

Если исправленной версии нет, исследуй реальное ограничение уязвимого поведения
и проверь его регрессионным тестом. Не скрывай advisory фильтром, удалением тестов,
подавлением ошибки или заменой нужной функции заглушкой. Если остаточный риск
не устранён, сообщи его явно и сохрани ошибку проверки.

Сбой сети, загрузки пакетов, базы или несовместимый scanner — не результат «уязвимостей
нет». Исправь окружение/причину и повтори аудит. Используй закреплённые инструменты
через `just`; не ставь произвольный `@latest` глобально. Основная проверка использует
text mode; дополнительный upstream query разбирает JSON и сам отвергает findings.
JSON/SARIF/OpenVEX могут завершаться с кодом 0 даже при наличии находок.

Результат привязан ко времени, платформе и известным базе проблемам. Он не доказывает
отсутствие неизвестных уязвимостей или полноту анализа reflection/unsafe.
См. [официальную документацию govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck).
