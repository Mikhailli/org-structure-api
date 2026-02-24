# Org Structure API

API для управления организационной структурой компании: подразделениями и сотрудниками.

## Технологии

- **Go 1.23** — язык программирования
- **net/http** — HTTP сервер (стандартная библиотека)
- **GORM** — ORM для работы с БД
- **PostgreSQL 16** — база данных
- **Goose** — миграции БД
- **Docker & Docker Compose** — контейнеризация

## Быстрый старт

### Запуск через Docker Compose

```bash
# Клонировать репозиторий
git clone <repository-url>
cd org-structure-api

# Запустить приложение
docker-compose up --build

# Приложение доступно по адресу http://localhost:8080
```

### Остановка

```bash
docker-compose down

# С удалением данных
docker-compose down -v
```

## Структура проекта

```
.
├── cmd/
│   └── api/
│       └── main.go           # Точка входа
├── internal/
│   ├── config/               # Конфигурация
│   ├── domain/               # Доменные модели и ошибки
│   ├── dto/                  # Data Transfer Objects
│   ├── handler/              # HTTP handlers и роутинг
│   ├── middleware/           # HTTP middleware
│   ├── repository/           # Слой работы с БД
│   └── service/              # Бизнес-логика
├── migrations/               # SQL миграции (Goose)
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## API Endpoints

### Подразделения

#### Создать подразделение
```
POST /departments/
Content-Type: application/json

{
  "name": "IT Department",
  "parent_id": null
}
```

#### Получить подразделение
```
GET /departments/{id}?depth=2&include_employees=true
```

Query параметры:
- `depth` (int, default: 1, max: 5) — глубина вложенных подразделений
- `include_employees` (bool, default: true) — включать сотрудников

#### Обновить подразделение
```
PATCH /departments/{id}
Content-Type: application/json

{
  "name": "New Name",
  "parent_id": 2
}
```

#### Удалить подразделение
```
DELETE /departments/{id}?mode=cascade
DELETE /departments/{id}?mode=reassign&reassign_to_department_id=2
```

Query параметры:
- `mode` (string, required) — режим удаления:
  - `cascade` — удалить подразделение, сотрудников и все дочерние
  - `reassign` — переназначить сотрудников в другое подразделение
- `reassign_to_department_id` (int) — ID целевого подразделения (при mode=reassign)

### Сотрудники

#### Создать сотрудника
```
POST /departments/{id}/employees/
Content-Type: application/json

{
  "full_name": "Иван Иванов",
  "position": "Backend Developer",
  "hired_at": "2024-01-15"
}
```

### Health Check

```
GET /health
```

## Примеры использования

### Создание структуры

```bash
# Создать корневое подразделение
curl -X POST http://localhost:8080/departments/ \
  -H "Content-Type: application/json" \
  -d '{"name": "Компания"}'

# Создать дочернее подразделение
curl -X POST http://localhost:8080/departments/ \
  -H "Content-Type: application/json" \
  -d '{"name": "IT отдел", "parent_id": 1}'

# Добавить сотрудника
curl -X POST http://localhost:8080/departments/2/employees/ \
  -H "Content-Type: application/json" \
  -d '{"full_name": "Иван Иванов", "position": "Developer"}'

# Получить структуру с глубиной 3
curl http://localhost:8080/departments/1?depth=3
```

### Перемещение подразделения

```bash
curl -X PATCH http://localhost:8080/departments/3 \
  -H "Content-Type: application/json" \
  -d '{"parent_id": 2}'
```

### Удаление с переназначением

```bash
curl -X DELETE "http://localhost:8080/departments/3?mode=reassign&reassign_to_department_id=2"
```

## Бизнес-правила

1. **Уникальность имени** — имя подразделения уникально в пределах родителя
2. **Защита от циклов** — нельзя переместить подразделение в своего потомка
3. **Валидация данных**:
   - Имя подразделения: 1-200 символов
   - ФИО сотрудника: 1-200 символов
   - Должность: 1-200 символов
4. **Каскадное удаление** — при удалении подразделения удаляются все дочерние и сотрудники

## Разработка

### Локальный запуск

```bash
# Запустить PostgreSQL
docker-compose up -d postgres

# Установить зависимости
go mod download

# Запустить приложение
DB_HOST=localhost go run ./cmd/api
```

### Запуск тестов

```bash
go test -v ./...
```

### Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| SERVER_PORT | 8080 | Порт HTTP сервера |
| DB_HOST | localhost | Хост PostgreSQL |
| DB_PORT | 5432 | Порт PostgreSQL |
| DB_USER | postgres | Пользователь БД |
| DB_PASSWORD | postgres | Пароль БД |
| DB_NAME | orgstructure | Имя базы данных |
| DB_SSLMODE | disable | SSL режим |

## Лицензия

MIT
