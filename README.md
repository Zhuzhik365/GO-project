# GO-project — ЛР3 Docker

ЛР3 сделана поверх ЛР2: сервер из ЛР1 сохранён, работа с PostgreSQL из ЛР2 сохранена, добавлены регистрация пользователя и авторизация с JWT. Теперь и Go-сервер, и PostgreSQL запускаются через Docker Compose.

## Что осталось от ЛР1

- `GET /test` проходит через 3 слоя `handler -> service -> repository`.
- `/test` возвращает ровно:

```text
Hello!
```

- Для `/test` разрешён только `GET`; другие методы возвращают `405 Method Not Allowed`.
- Сохранён graceful shutdown по `SIGINT` / `SIGTERM`.

## Что осталось от ЛР2

- PostgreSQL подключается при старте сервера.
- Таблицы инициализируются на слое `repository`.
- `POST /dbtest` записывает строку из тела запроса в таблицу `db_test`.
- Всё запускается через Docker Compose: `mini-avito-app` + `mini-avito-postgres`.

## Что добавлено в ЛР3

- Таблица `users` используется для хранения пользователей.
- `POST /users/register` создаёт пользователя в БД.
- `POST /users/login` проверяет логин/пароль и возвращает JWT-токен.
- Работа с БД находится в `repository`.
- Бизнес-логика регистрации, проверки пароля и генерации JWT находится в `service`.
- HTTP-обработка JSON-запросов находится в `handler`.

Дополнительно добавлены короткие алиасы:

- `POST /register`
- `POST /login`

## Запуск всей ЛР3 одной командой

```bash
docker compose up --build
```

Если нужно запустить в фоне:

```bash
docker compose up --build -d
```

Проверить контейнеры:

```bash
docker compose ps
```

Должны быть запущены:

```text
mini-avito-app
mini-avito-postgres
```

## Адреса

Go-сервер работает на порту `8080`:

```text
http://localhost:8080
```

PostgreSQL работает на порту `5432`, но его не нужно открывать в браузере.

В GitHub Codespaces открывать нужно ссылку именно с `8080`, например:

```text
https://...-8080.app.github.dev/test
```

Не открывать `5432` в браузере — это порт базы данных.

## Проверка ЛР1

```bash
curl -i http://localhost:8080/test
```

Ожидаемый ответ:

```text
HTTP/1.1 200 OK

Hello!
```

## Проверка ЛР2

```bash
curl -i -X POST http://localhost:8080/dbtest -d "hello database"
```

Ожидаемый ответ:

```text
HTTP/1.1 201 Created
Content-Type: application/json
```

И JSON примерно такого вида:

```json
{"id":1,"body":"hello database","created_at":"..."}
```

Проверить записи в БД:

```bash
docker exec -it mini-avito-postgres psql -U postgres -d mini_avito -c "SELECT * FROM db_test ORDER BY id DESC;"
```

## Проверка ЛР3: регистрация пользователя

```bash
curl -i -X POST http://localhost:8080/users/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"testuser@example.com","password":"123456"}'
```

Ожидаемый ответ:

```text
HTTP/1.1 201 Created
Content-Type: application/json
```

И JSON примерно такого вида:

```json
{"id":1,"username":"testuser","email":"testuser@example.com","created_at":"..."}
```

## Проверка ЛР3: авторизация и получение JWT

```bash
curl -i -X POST http://localhost:8080/users/login \
  -H "Content-Type: application/json" \
  -d '{"login":"testuser","password":"123456"}'
```

Ожидаемый ответ:

```text
HTTP/1.1 200 OK
Content-Type: application/json
```

И JSON с токеном:

```json
{
  "token": "JWT_TOKEN_HERE",
  "expires_at": "...",
  "user": {
    "id": 1,
    "username": "testuser",
    "email": "testuser@example.com",
    "created_at": "..."
  }
}
```

## Проверить пользователей в БД

```bash
docker exec -it mini-avito-postgres psql -U postgres -d mini_avito -c "SELECT id, username, email, created_at FROM users ORDER BY id DESC;"
```

## Остановить проект

```bash
docker compose down
```

Если нужно удалить ещё и данные БД:

```bash
docker compose down -v
```

Это полезно, если хочешь заново зарегистрировать пользователя с тем же username/email.

## Что говорить на защите

В ЛР1 был простой HTTP-сервер с `/test`, чистой архитектурой и graceful shutdown. В ЛР2 мы добавили PostgreSQL, инициализацию таблиц в repository и `/dbtest`, который пишет данные в БД. В ЛР3 мы добавили регистрацию и авторизацию: пользователь сохраняется в таблицу `users`, пароль хранится в виде хэша, а при логине сервер возвращает JWT-токен. Всё приложение теперь запускается через Docker Compose: отдельный контейнер для Go-сервера и отдельный контейнер для PostgreSQL.
