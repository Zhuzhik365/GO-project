Регистрация нового пользователя (POST):
curl -X POST http://localhost:8080/users/register -H "Content-Type: application/json" -d '{"username":"user1","email":"user1@example.com","password":"password123"}'

Вход пользователя (POST):
curl -X POST http://localhost:8080/users/login -H "Content-Type: application/json" -d '{"login":"user1","password":"password123"}'

Создание нового объявления (POST):
curl -X POST http://localhost:8080/ads/create -H "Authorization: Bearer <ваш_токен>" -H "Content-Type: application/json" -d '{"title":"Название","description":"Описание","price":100.50}'

Получение списка своих объявлений (GET):
curl -X GET http://localhost:8080/ads -H "Authorization: Bearer <ваш_токен>"

Запуск проекта со сборкой (в интерактивном режиме):
docker-compose up --build

Запуск проекта в фоновом режиме:
docker-compose up --build -d
