# auction-router

Роутер аукционных запросов: принимает от площадки описание рекламного места, отбирает по
претаргетингу подходящих DSP-партнёров, опрашивает их параллельно с общим таймаутом и возвращает
результат.

Только стандартная библиотека Go 1.26, без внешних зависимостей.

## Запуск

```sh
go run ./cmd/router
go test -race ./...
```

Если установлен [mise](https://mise.jdx.dev/), есть таски: `mise run run`, `mise run check` (vet,
gofmt, тесты под `-race`), `mise run cover`, `mise run build`.

Доступные флаги:

| флаг         | по умолчанию    | что задаёт                             |
|--------------|-----------------|----------------------------------------|
| `-addr`      | `:8080`         | адрес HTTP-сервера                     |
| `-partners`  | `partners.json` | файл с партнёрами                      |
| `-budget`    | `200ms`         | таймаут на один раунд опроса партнёров |
| `-log-level` | `info`          | `debug`, `info`, `warn` или `error`    |

По `Ctrl-C` или `SIGTERM` сервер перестаёт принимать соединения и даёт текущим аукционам до пяти
секунд на завершение.

## Эндпоинты

`POST /auction` — аукцион. Тело должно быть объявлено как `application/json`, иначе 415; размер тела
ограничен 64 КиБ, иначе 413.

`GET /health` — 204 без тела.

## Фильтрация: пример

Один и тот же лот, в котором меняется по одному полю. Каждый раз исключается другой партнёр.
Настройки партнёров описаны в разделе [Партнёры](#партнёры).

Базовый запрос — проходят `dsp-alpha` и `dsp-delta`:

```sh
curl -s -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"r-1","country":"RU","device_type":"mobile","bid_floor":1.5}'
```

```json
{
  "request_id": "r-1",
  "status": "ok",
  "matched_dsps": [
    "dsp-alpha",
    "dsp-delta"
  ],
  "sent": 2,
  "succeeded": 1,
  "duration_ms": 0
}
```

Добавил категорию `gambling` — `dsp-alpha` её блокирует. Остался `dsp-delta` с окончанием `/error`,
отсюда `succeeded: 0`: партнёр ответил ошибкой.

```sh
curl -s -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"r-2","country":"RU","device_type":"mobile","bid_floor":1.5,"categories":["gambling"]}'
```

```json
{
  "request_id": "r-2",
  "status": "ok",
  "matched_dsps": [
    "dsp-delta"
  ],
  "sent": 1,
  "succeeded": 0,
  "duration_ms": 0
}
```

Сменил устройство на desktop — `dsp-delta` покупает только mobile и tv и выпадает:

```sh
curl -s -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"r-3","country":"RU","device_type":"desktop","bid_floor":1.5}'
```

```json
{
  "request_id": "r-3",
  "status": "ok",
  "matched_dsps": [
    "dsp-alpha"
  ],
  "sent": 1,
  "succeeded": 1,
  "duration_ms": 0
}
```

Поднял цену до 3.0 — прошёл порог `dsp-gamma`, партнёров стало трое:

```sh
curl -s -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"r-4","country":"RU","device_type":"mobile","bid_floor":3.0}'
```

```json
{
  "request_id": "r-4",
  "status": "ok",
  "matched_dsps": [
    "dsp-alpha",
    "dsp-gamma",
    "dsp-delta"
  ],
  "sent": 3,
  "succeeded": 2,
  "duration_ms": 0
}
```

Сменил страну на DE — сюда попадает `dsp-omega`, который не отвечает никогда. Раунд заканчивается по
бюджету, ответ всё равно отдан:

```sh
curl -s -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"r-5","country":"DE","device_type":"mobile","bid_floor":1.5}'
```

```json
{
  "request_id": "r-5",
  "status": "ok",
  "matched_dsps": [
    "dsp-beta",
    "dsp-omega"
  ],
  "sent": 2,
  "succeeded": 1,
  "duration_ms": 201
}
```

Никто не подошёл — 200 с пустым списком и статусом `no_matched_dsps`:

```sh
curl -s -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"r-6","country":"JP","device_type":"tv","bid_floor":99}'
```

```json
{
  "request_id": "r-6",
  "status": "no_matched_dsps",
  "matched_dsps": [],
  "sent": 0,
  "succeeded": 0,
  "duration_ms": 0
}
```

Некорректный запрос — 400 со всеми ошибками сразу:

```sh
curl -s -X POST localhost:8080/auction \
  -H 'Content-Type: application/json' \
  -d '{"request_id":"r-7","country":"russia","device_type":"fridge","bid_floor":-1}'
```

```json
{
  "error": "request does not describe a lot the exchange can sell",
  "details": [
    "country: \"russia\" is not an ISO 3166-1 alpha-2 code",
    "device_type: \"fridge\" is not a known device type",
    "bid_floor: -1 is negative"
  ]
}
```

Исключение по `is_enabled` в ответе не видно: выключенный партнёр не попадает в `matched_dsps`. Его,
как и причины исключения всех остальных, показывает debug-лог — раздел [Логи](#логи).

## Партнёры

Лежат в [`partners.json`](./partners.json), путь задаётся флагом `-partners`. Шесть штук, подобраны
так, чтобы каждое правило претаргетинга отсекало кого-нибудь:

| uid           | окончание endpoint | включён | страны | устройства      | min_bid_floor | блокирует |
|---------------|--------------------|---------|--------|-----------------|---------------|-----------|
| `dsp-alpha`   | `/ok`              | да      | RU, KZ | mobile, desktop | 0.5           | gambling  |
| `dsp-beta`    | `/ok`              | да      | US, DE | любые           | 0.1           | —         |
| `dsp-gamma`   | `/ok`              | да      | любые  | mobile          | 2.0           | —         |
| `dsp-delta`   | `/error`           | да      | RU     | mobile, tv      | 0             | —         |
| `dsp-omega`   | `/slow`            | да      | DE     | любые           | 0             | —         |
| `dsp-epsilon` | `/ok`              | нет     | любые  | любые           | 0             | —         |

У `dsp-epsilon` ограничений нет ни по одному полю, так что подошёл бы под любой запрос, но
`is_enabled: false` исключает его.

Новый партнёр — ещё один объект в массиве. Обязательных полей четыре:

```json
{
  "uid": "dsp-new",
  "name": "DSP New",
  "endpoint": "https://new.dsp.example/ok",
  "is_enabled": true
}
```

Остальные поля необязательны, и пропуск означает отсутствие ограничения: нет списка стран — партнёр
работает со всеми, нет `min_bid_floor` — покупает по любой цене.

`is_enabled` обязателен намеренно: `encoding/json` молча игнорирует незнакомые ключи, поэтому при
умолчании опечатка вроде `"enabled"` тихо включила бы или выключила партнёра.

Файл разбирается целиком при старте, ошибки сообщаются разом, с адресом каждой:

```
reading partners from partners.json:
partners[2].uid: "" is empty or padded with whitespace
partners[3].countries[1]: "RUS" is not an ISO 3166-1 alpha-2 code
partners[5].uid: uid is claimed twice, first by partners[1]
```

Окончание адреса задаёт поведение заглушки: `/slow` молчит до конца раунда, `/error` возвращает
ошибку, любое другое (включая `/ok` и `/bid`) отвечает сразу.

## Архитектура

```
cmd/router          конфигурация, сборка, мультиплексор, graceful shutdown
internal/auction    предметная область: лот, партнёр, реестр, претаргетинг, биржа
internal/api        HTTP: разбор запроса, хендлер, сериализация ответа, логи
internal/jsonfile   чтение партнёров из файла
internal/parsing    общее между разбором запроса и разбором файла
internal/fakedsp    заглушка вместо партнёров
```

Зависимости направлены внутрь. `internal/auction` не знает про HTTP, про JSON и про то, откуда
взялись партнёры; `internal/api` не знает правил претаргетинга. Про файл знает только
`internal/jsonfile`, и упомянут он в одной строке `cmd/router`.

Интерфейс `PartnerSource` объявлен у потребителя, в `cmd/router`, а не рядом с реализацией. Чтобы
перейти на SQL, нужно реализовать `Load(ctx) ([]auction.Partner, error)` и поменять одну строку.

Претаргетинг — `Partner.Consider`. Возвращает первое непройденное условие (`Term`), а не
`bool`, поэтому одна функция даёт и отбор, и причину исключения для лога.

Опрос партнёров — `Exchange.canvass`. Горутина на партнёра, один общий дедлайн на весь
раунд. Паника внутри опроса превращается в ошибку. Если дедлайн вызывающего меньше бюджета, раунд
заканчивается по нему.

## Логи

Одна строка на запрос, уровень `info`:

```jsonl
{"level":"INFO","msg":"auction held","request_id":"r-1","country":"RU","device_type":"mobile","matched":2,"filtered":4,"succeeded":1,"duration_ms":0}
```

Причина исключения конкретного партнёра — на `debug`. Строк тут столько же, сколько
партнёров: при шести партнёрах и 100k RPS это 600 тысяч строк в секунду.

```sh
go run ./cmd/router -log-level debug
```

```jsonl
{"level":"DEBUG","msg":"partner passed over","request_id":"r-1","partner":"dsp-beta","term":"country"}
{"level":"DEBUG","msg":"partner passed over","request_id":"r-1","partner":"dsp-gamma","term":"bid_floor"}
{"level":"DEBUG","msg":"partner passed over","request_id":"r-1","partner":"dsp-omega","term":"country"}
{"level":"DEBUG","msg":"partner passed over","request_id":"r-1","partner":"dsp-epsilon","term":"enabled"}
{"level":"WARN","msg":"partner did not answer","request_id":"r-1","partner":"dsp-delta","error":"the partner answered with an error"}
```

Молчание партнёра — `warn`, а не `debug`: исключение по претаргетингу работает штатно, а здесь
отказ на стороне партнёра.

## Решения

- DSP-клиент — интерфейс с заглушкой. Задание допускает три варианта, выбрал третий:
  `auction.Inviter` и фейковая реализация в `internal/fakedsp`, которая отвечает по
  окончанию адреса. Реального HTTP-запроса к партнёру нет. Параллельность, общий таймаут
  и устойчивость к отказу одного партнёра покрыты тестами `internal/auction` и
  воспроизводятся на живом сервисе примерами выше.
- В ответе есть `status`, которого нет в форме из задания. Задание требует явно
  указывать, что отправлять было некому. Добавил поле со значениями `ok` и
  `no_matched_dsps` и заполняю его в любом успешном ответе, а не только в пустом.
- `sent` всегда равен длине `matched_dsps`. Здесь запрос уходит каждому, кто прошёл
  претаргетинг. В реальной системе числа разошлись бы, например, из-за rate limit на партнёра.
- Страна проверяется по форме, а не по справочнику — две заглавные латинские буквы.
- `succeeded` считает ответы, а не ставки. Разбор ответа партнёра задание не
  требует, поэтому успехом считается сам факт ответа: отказ от покупки здесь
  неотличим от ставки.
- `Content-Type` обязателен. Без `application/json` — 415, без заголовка вовсе —
  тоже. Параметры вроде `; charset=utf-8` игнорируются.
- Тело ограничено 64 КиБ, дальше 413. У `net/http` по умолчанию нет ограничения на
  тело, только на заголовки, так что один клиент, отправляющий бесконечную строку, занял бы
  память сервера целиком.
- `GET /health` без тела. Код состояния и есть весь ответ. Отдельный readiness не
  делал: партнёры читаются один раз при старте, и при ошибке чтения сервис не стартует
  вовсе, так что readiness совпал бы с liveness.

## Что доделал бы

Из необязательного в задании сделаны health-эндпоинт, graceful shutdown и конфигурация флагами.

- Ограничение числа одновременных приглашений. Сейчас в `canvass` на каждого
  подошедшего партнёра заводится горутина, и потолка у их числа нет — ни на один
  аукцион, ни на сервис целиком.
- Метрики. Счётчики по партнёрам и причинам исключений, гистограмма длительности
  раунда, доля ответивших.
- Circuit breaker и rate limit на партнёра. Стабильно молчащий партнёр не должен
  получать запросы и тратить бюджет раунда.
- Бенчмарк претаргетинга. `Roster.Consider` линейно обходит всех партнёров на каждый
  запрос — с их ростом это узкое место; смотрел бы в сторону инвертированного индекса
  country -> партнёры, но сначала измерил бы.
- Перезагрузка партнёров без рестарта по `SIGHUP`.
- OpenRTB вместо самодельного формата. Вместе с ним приходят закрытые словари — категория сейчас
  любая непустая строка, поэтому `"News"` и `"news"` не совпадают, а в OpenRTB это таксономия IAB.
- Настоящий HTTP-клиент вместо заглушки. Задание разрешает обойтись фейком, но реального сетевого
  кода в проекте из-за этого нет.
- Партнёры в БД с миграциями и Dockerfile. Необязательные пункты задания, до которых
  не дошёл; хранилище изолировано интерфейсом `PartnerSource` — он читает партнёров при
  старте, а не на каждый аукцион, так что смена затрагивает одну строку в `cmd/router`.
