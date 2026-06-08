### Windows (PowerShell)

Используем родной `Invoke-RestMethod`. Он автоматически красиво форматирует JSON в консоли.

**Важно:** Перед началом работы один раз введи команду для включения UTF-8, чтобы русские буквы отображались корректно:

```powershell
chcp 65001

```

**1. Регистрация нового пользователя:**

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/users/register" -ContentType "application/json; charset=utf-8" -Body '{"username":"avito_seller", "email":"seller@example.com", "password":"password123"}'

```

**2. Авторизация и получение JWT-токена:**

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/users/login" -ContentType "application/json; charset=utf-8" -Body '{"login":"avito_seller", "password":"password123"}'

```

**3. Создание нового объявления:**

```powershell
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/ads/create" -ContentType "application/json; charset=utf-8" -Body '{"user_id":1, "title":"Ноутбук Apple MacBook Pro", "description":"В отличном состоянии, батарея держит долго.", "price":95000.00}'

```

**4. Получение списка всех объявлений пользователя (по user_id):**

```powershell
Invoke-RestMethod -Method Get -Uri "http://localhost:8080/ads?user_id=1"

```

---

### 🐧 Linux / GitHub Codespaces (Bash)

Используем классический `curl`. Тело JSON обязательно оборачиваем в одинарные кавычки `' '`, чтобы терминал не ломал синтаксис. Команды вытянуты в одну строку для надежного копипаста.

**1. Регистрация нового пользователя:**

```bash
curl -i -X POST http://localhost:8080/users/register -H "Content-Type: application/json" -d '{"username":"avito_seller", "email":"seller@example.com", "password":"password123"}'

```

**2. Авторизация и получение JWT-токена:**

```bash
curl -i -X POST http://localhost:8080/users/login -H "Content-Type: application/json" -d '{"login":"avito_seller", "password":"password123"}'

```

**3. Создание нового объявления:**

```bash
curl -i -X POST http://localhost:8080/ads/create -H "Content-Type: application/json" -d '{"user_id":1, "title":"Ноутбук Apple MacBook Pro", "description":"В отличном состоянии, батарея держит долго.", "price":95000.00}'

```

**4. Получение списка всех объявлений пользователя (по user_id):**

```bash
curl -i -X GET "http://localhost:8080/ads?user_id=1"

```

---

### 🐳 Docker и База Данных PostgreSQL

Эти команды универсальны и работают **одинаково** как в Windows, так и в GitHub Codespaces.

**Управление контейнерами:**

```bash
# Запуск сервиса и базы данных в фоновом режиме
docker compose up --build -d

# Остановка контейнеров
docker compose down

# Полная очистка (остановка + удаление базы данных, чтобы начать с чистого листа)
docker compose down -v

```

**Просмотр данных напрямую в БД:**

```bash
# Посмотреть всех зарегистрированных пользователей
docker exec -it mini-avito-postgres psql -U postgres -d mini_avito -c "SELECT id, username, email FROM users;"

# Посмотреть все созданные объявления
docker exec -it mini-avito-postgres psql -U postgres -d mini_avito -c "SELECT id, user_id, title, price, status FROM ads;"

```
