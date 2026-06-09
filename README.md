# GO-project: Mini-Avito на Go

Проект сделан как основная часть сервера по лабораторным работам ЛР1–ЛР5 и расширен под оценку выше базовой: добавлен отдельный сервис обработки объявлений через RabbitMQ, а также простой мониторинг через Prometheus + Grafana.

## Что есть в проекте

### ЛР1

- `GET /test`
- Ответ: `Hello!`
- Чистая архитектура: `handler -> service -> repository`
- Graceful shutdown

### ЛР2

- PostgreSQL
- Инициализация таблиц при старте
- `POST /dbtest`
- Запись строки из тела запроса в таблицу `db_test`

### ЛР3

- `POST /users/register`
- `POST /users/login`
- Пользователь сохраняется в БД
- Пароль хранится как хэш, а не открытым текстом
- При логине выдаётся JWT

### ЛР4

- `POST /ads/create`
- `GET /ads`
- Создание объявления
- Просмотр своих объявлений и их статусов

### ЛР5

- JWT middleware
- Защищённые ручки требуют заголовок `Authorization: Bearer <token>`
- Middleware проверяет JWT, достаёт `user_id` и передаёт его в хэндлеры через `context`
- Пользователь видит только свои объявления

### Дополнение для оценки 6–7

Добавлен отдельный сервис обработки объявлений через RabbitMQ:

1. Основной сервис создаёт объявление со статусом `pending`.
2. Основной сервис отправляет сообщение в RabbitMQ в очередь `ads.created`.
3. Отдельный сервис `processor` получает сообщение и имитирует обработку.
4. `processor` отправляет сообщение об изменении статуса в очередь `ads.status.changed`.
5. Основной сервис получает это сообщение и меняет статус объявления на `active`.

Для Mini-Avito это аналог сервиса обработки заказов из методички: вместо заказа обрабатывается объявление.

### Дополнение для оценки 8

Добавлены:

- `Prometheus`
- `Grafana`
- `/metrics` у Go-сервера
- Docker Compose deployment для всех сервисов

## Запуск

```bash
docker compose up --build
```

Или в фоне:

```bash
docker compose up --build -d
```

Проверить контейнеры:

```bash
docker compose ps
```

## Адреса сервисов

- Go API: `http://localhost:8080`
- RabbitMQ Management: `http://localhost:15672`
  - login: `guest`
  - password: `guest`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`
  - login: `admin`
  - password: `admin`

## Проверка ЛР1

```bash
curl -i http://localhost:8080/test
```

Ожидаемо:

```text
HTTP/1.1 200 OK

Hello!
```

## Проверка ЛР2

```bash
curl -i -X POST http://localhost:8080/dbtest -d "test message"
```

Ожидаемо:

```json
{"id":1,"body":"test message","created_at":"..."}
```

Вывести записи из БД:

```bash
docker exec -it mini-avito-postgres psql -U postgres -d mini_avito \
  -c "SELECT * FROM db_test ORDER BY id DESC;"
```

## Проверка ЛР3

Регистрация:

```bash
curl -i -X POST http://localhost:8080/users/register \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","email":"user1@example.com","password":"password123"}'
```

Логин:

```bash
curl -i -X POST http://localhost:8080/users/login \
  -H "Content-Type: application/json" \
  -d '{"login":"user1","password":"password123"}'
```

В ответе будет поле `token`. Его нужно скопировать.

Можно сразу сохранить токен в переменную:

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/users/login \
  -H "Content-Type: application/json" \
  -d '{"login":"user1","password":"password123"}' \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')

echo $TOKEN
```

## Проверка ЛР4–ЛР5

Без токена ручка не должна пускать:

```bash
curl -i http://localhost:8080/ads
```

Ожидаемо:

```text
HTTP/1.1 401 Unauthorized
```

Создать объявление с JWT:

```bash
curl -i -X POST http://localhost:8080/ads/create \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"iPhone 13","description":"Good phone","price":45000}'
```

Сразу после создания статус будет `pending`:

```json
{
  "id": 1,
  "user_id": 1,
  "title": "iPhone 13",
  "description": "Good phone",
  "price": 45000,
  "status": "pending",
  "created_at": "...",
  "updated_at": "..."
}
```

Подождать несколько секунд и получить свои объявления:

```bash
sleep 5
curl -i http://localhost:8080/ads \
  -H "Authorization: Bearer $TOKEN"
```

После обработки отдельным сервисом статус должен стать `active`.

## Проверка RabbitMQ / сервиса обработки

Посмотреть логи обработчика:

```bash
docker logs mini-avito-processor
```

Там должно быть примерно:

```text
new ad received: ad_id=1 user_id=1 title="iPhone 13"
status changed event sent: ad_id=1 status=active
```

Посмотреть статус в БД:

```bash
docker exec -it mini-avito-postgres psql -U postgres -d mini_avito \
  -c "SELECT id, user_id, title, status, created_at, updated_at FROM ads ORDER BY id DESC;"
```

## Проверка Prometheus / Grafana

Метрики приложения:

```bash
curl http://localhost:8080/metrics
```

Prometheus:

```text
http://localhost:9090
```

В Prometheus можно выполнить запрос:

```text
mini_avito_http_requests_total
```

Grafana:

```text
http://localhost:3000
```

Логин/пароль:

```text
admin / admin
```

В Grafana уже добавлен datasource Prometheus и dashboard `Mini Avito`.

## Архитектура

```text
HTTP request
    ↓
handler
    ↓
service
    ↓
repository
    ↓
PostgreSQL
```

Для асинхронной обработки:

```text
POST /ads/create
    ↓
main app создаёт объявление pending
    ↓
main app -> RabbitMQ queue ads.created
    ↓
processor получает сообщение
    ↓
processor имитирует обработку
    ↓
processor -> RabbitMQ queue ads.status.changed
    ↓
main app получает событие
    ↓
main app обновляет статус объявления на active
```

## Что говорить на защите

В основной части проекта реализованы все 5 лабораторных работ. Первая лабораторная — HTTP-сервер с `/test`, чистой архитектурой и graceful shutdown. Во второй добавлена PostgreSQL и `/dbtest`, который пишет данные в БД. В третьей добавлены регистрация, логин и JWT. В четвёртой добавлена работа с объявлениями: создание и просмотр своих объявлений. В пятой добавлен JWT middleware, который проверяет токен и передаёт `user_id` в хэндлеры через `context`.

Для повышения оценки добавлен отдельный сервис обработки объявлений через RabbitMQ. Основной сервис отправляет событие о создании объявления, отдельный сервис обрабатывает его и отправляет событие об изменении статуса, после чего основной сервис обновляет статус объявления в PostgreSQL. Также добавлен Prometheus/Grafana deployment через Docker Compose.
