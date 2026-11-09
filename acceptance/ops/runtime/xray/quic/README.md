# Потеря PMTU-пробы в расширенном congestion callback

`just quic-mtu-congestion-test build/quic-mtu-regression/<name>` создаёт новые
control/patched копии закреплённого QUIC-модуля, воспроизводит дефект и проверяет
локальное исправление. Команда не меняет module cache, checkout форка, runtime
pins или установленный движок. Существующий output не перезаписывается.

База — `github.com/Deliner/vpn-dep-apernet-quic-go`
`v0.61.1-0.20261109033109-84023edbb399`, commit
`84023edbb3990540fb9cbc22d4b303eecfd1c353`, module sum
`h1:gvzU9Neeo6OJm396ji4Og8XI7QpsO+iyuIWfHxZdWLk=`.
Источник выбирается через закреплённый Xray module graph; download metadata,
проверка cache, полные копии source, logs и SHA inventories остаются в output.
Контрольные тесты используют исходный Go graph самого QUIC-модуля.

## Дефект и граница исправления

`detectLostPackets` исключает потерянную PMTU-пробу из обычного
`OnCongestionEvent`, но добавляет её в `lostPacketsInfo` расширенного callback.
BBR использует этот расширенный путь. Поэтому одна и та же потеря получает
разный смысл в двух API. [RFC 9000 §14.4](https://www.rfc-editor.org/rfc/rfc9000.html#section-14.4)
рекомендует не считать потерю PMTU-пробы сигналом перегрузки: пакет мог быть
отброшен из-за проверяемого размера. При этом проба учитывается в bytes-in-flight.

`mtu-probe-loss.patch` исключает probe loss из расширенного списка и вычитает
discarded probe bytes из его исходного in-flight значения. Иначе контроллер,
вычитающий только перечисленные ACK/loss bytes, сохранит уже удалённые bytes.
Оба входа — обработка ACK и loss timer — используют одинаковое правило.
Сам PMTU loss callback, обычные потери, delivery ACK пробы и packet history
сохраняются. Public интерфейсы и congestion policy не меняются.

## Проверка

`mtu-probe-tests.patch` добавляет реальные ACK-handler сценарии с observer
расширенного callback поверх штатного congestion controller:

- ACK одновременно подтверждает data и обнаруживает data/probe losses;
- loss timer обнаруживает только PMTU loss либо смешанные потери;
- late ACK не вызывает повторного учёта потерянных пакетов;
- успешный ACK PMTU-пробы продолжает учитываться как delivery;
- in-flight значение после расширенного batch равно состоянию handler.

Control обязан завершиться именно ожидаемыми assertions о PMTU loss; ошибка
сборки, panic или data race не считается воспроизведением. Patched выполняет
20 повторов этих тестов с race detector, затем полные `internal/ackhandler`
и `internal/congestion` без cache. В каждом повторе четыре leaf test cases.

Первый воспроизводимый run — `build/quic-mtu-regression/probe-loss-001`:
control обнаружил оба ошибочных callback пути, patched прошёл 80 leaf cases
и оба полных пакета на Darwin ARM64 / Go 1.27.3. Это подтверждает дефект
контракта QUIC, но ещё не доказывает причину jitter timeouts MASQUE, эффект
на пропускную способность или готовность к публикации.

## Native Linux и парные Xray-кандидаты

`just quic-mtu-congestion-linux-test <regression> build/quic-mtu-native/<name>`
перепроверяет полный source/evidence inventory и соответствие maintained patch,
компилирует оба ACK-handler test binaries и congestion package для Linux.
В принадлежащей проекту VM проверяются hostname, CPU, пользователь и input SHA;
общий payload lock исключает параллельные сетевые испытания. После возврата и
проверки reports удаляются только три guest-копии test binaries. Локальные inputs
сохраняются. Это native execution с CGO=0, не Linux race run.

Runs `probe-loss-arm64-001` и `probe-loss-arm64-002` подтвердили три ожидаемых
control failures, по 80 успешных patched leaf cases и оба полных пакета.
Повтор проверил общую верификацию inputs, выделенную в `inputs.sh` для двух
потребителей: native runner и candidate builder. Бинарники повторов совпали.

`just quic-mtu-candidate <base> <regression> control|patched build/runtime-candidates/<name>`
создаёт новую экспериментальную сборку Xray. Полный QUIC source с лицензиями
включается в `source/nhop-dependencies/apernet-quic-go`; go.mod использует relative
replacement. Обе сборки имеют одинаковые graph, tags и build label; оригинальные
candidate и regression inputs повторно проверяются после сборки. BBR и Brutal
consumer tests выполняются с race detector на build host. Pins не меняются.

Первая ARM64-пара — `xray-quic-mtu-{control,patched}-arm64`. В каждой 1729 source
files; единственное различие source между вариантами — `sent_packet_handler.go`.
SHA-256 control binary:
`8fe7529a97c64813b92ea2ed30c270a9874f1b6a1c7c5d56bbb789ecf57e4b08`;
patched binary:
`4da41a14063615096cc7a768a11069b134c08d1578bc2fac188ef4fbd222b357`.
Независимая сверка полных inventories, Linux ELF/ARM64, результатов тестов и
изменённых исходников сохранена в
`build/maintenance/quic-mtu-congestion/native-candidates-integrity.json`.
Парное сетевое сравнение подтвердило, что патч **не устраняет** MASQUE jitter
timeouts: по два delay-history runs на вариант и по одному трёхцикловому fault
matrix run сохранили ошибки TCP в delay phases. Cold-start, clean recovery,
MTU и restart phases прошли. [Полные результаты](../../../../docs/development/runtime-masque.md)
отделяют доказанное исправление QUIC-контракта от незакрытой приёмки MASQUE H3.
