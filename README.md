# 💰 Finance Tracker

Система управления личными финансами построенная на микросервисной архитектуре. Демонстрирует межсервисное общение через gRPC, три типа gRPC вызовов, API Gateway паттерн и современный Go стек.

## 🏗️ Архитектура

```
Пользователь
     │ REST (JSON)
     ▼
API Gateway :8080          ← единственная точка входа (Gin)
     │
     ├── gRPC ──► Transaction Service :50051  ── PostgreSQL
     │             (транзакции, пользователи)
     │
     ├── gRPC ──► Budget Service :50052       ── PostgreSQL
     │             (бюджеты, лимиты)
     │
     └── gRPC ──► Report Service :50053       ── Redis (кэш)
                   (аналитика, прогнозы)
                        │
                        └── gRPC ──► Transaction Service
                                      (читает данные)
```

## 🚀 Стек технологий

| Слой | Технология |
|---|---|
| Язык | Go 1.24 |
| REST API | Gin |
| Межсервисное общение | gRPC + Protocol Buffers |
| База данных | PostgreSQL (отдельная для каждого сервиса) |
| Кэш | Redis |
| Авторизация | JWT |
| Контейнеризация | Docker + docker-compose |
| CI/CD | GitHub Actions |

## 📦 Сервисы

### API Gateway
Единственная точка входа. Принимает REST запросы, проверяет JWT и вызывает нужные gRPC сервисы.

### Transaction Service
Хранит все финансовые операции и пользователей. Реализует Server Streaming для истории транзакций.

### Budget Service
Следит за лимитами расходов по категориям. При достижении 80% лимита — предупреждение, при 100% — алерт. Реализует Bidirectional Streaming для алертов в реальном времени.

### Report Service
Считает аналитику на основе данных из Transaction Service. Кэширует результаты в Redis. Реализует Server Streaming для графиков расходов по дням.

## ⚡ Типы gRPC вызовов

| Тип | Где используется |
|---|---|
| **Unary** | создать транзакцию, создать бюджет, получить баланс |
| **Server Streaming** | история транзакций, график расходов по дням |
| **Bidirectional Streaming** | алерты о превышении бюджета |

## 🗂️ Структура проекта

```
finance-tracker/
├── proto/                         — контракты между сервисами
│   ├── transaction.proto
│   ├── budget.proto
│   └── report.proto
├── gen/                           — сгенерированный Go код
│   ├── transaction/
│   ├── budget/
│   └── report/
├── services/
│   ├── gateway/                   — REST API (Gin)
│   │   ├── client/                — gRPC клиенты
│   │   ├── handler/               — HTTP хендлеры
│   │   ├── middleware/            — JWT
│   │   └── main.go
│   ├── transaction/               — gRPC сервер
│   │   ├── grpc/                  — реализация gRPC методов
│   │   ├── repository/            — работа с БД
│   │   ├── migrations/
│   │   └── main.go
│   ├── budget/                    — gRPC сервер
│   │   ├── grpc/
│   │   ├── repository/
│   │   ├── migrations/
│   │   └── main.go
│   └── report/                    — gRPC сервер
│       ├── analytics/             — бизнес-логика отчётов
│       │   ├── analytics.go
│       │   ├── analytics_test.go
│       │   └── mocks/
│       ├── cache/                 — Redis кэш
│       ├── client/                — gRPC клиент Transaction
│       ├── grpc/
│       └── main.go
├── shared/
│   ├── config/                    — загрузка конфига
│   └── interceptor/               — gRPC middleware (логи, recover)
├── .github/workflows/ci.yml       — GitHub Actions
├── docker-compose.yml
└── README.md
```

## 🏃 Быстрый старт

### Через Docker (рекомендуется)

```bash
git clone https://github.com/Alexandr20i/finance-tracker.git
cd finance-tracker
docker-compose up --build
```

Поднимает все 7 контейнеров автоматически:
- 4 сервиса (gateway, transaction, budget, report)
- 2 базы данных PostgreSQL
- Redis

### Локально

**1. Создай базы данных**
```bash
psql -U postgres -c "CREATE DATABASE finance_transaction;"
psql -U postgres -c "CREATE DATABASE finance_budget;"
psql -U postgres -d finance_transaction -f services/transaction/migrations/001_init.sql
psql -U postgres -d finance_budget -f services/budget/migrations/001_init.sql
```

**2. Запусти Redis**
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

**3. Запусти сервисы в четырёх терминалах**
```bash
# Терминал 1
DB_NAME=finance_transaction go run ./services/transaction/main.go

# Терминал 2
DB_NAME=finance_budget go run ./services/budget/main.go

# Терминал 3
go run ./services/report/main.go

# Терминал 4
go run ./services/gateway/main.go
```

## 🔑 API

### Auth
| Метод | URL | Описание |
|-------|-----|----------|
| POST | `/auth/register` | Регистрация |
| POST | `/auth/login` | Вход |

### Транзакции
| Метод | URL | Описание |
|-------|-----|----------|
| POST | `/transactions` | Создать транзакцию |
| GET | `/transactions` | История (фильтры: from, to, category, type) |
| GET | `/transactions/:id` | Детали транзакции |
| DELETE | `/transactions/:id` | Удалить транзакцию |

### Бюджеты
| Метод | URL | Описание |
|-------|-----|----------|
| POST | `/budget` | Создать бюджет на категорию |
| GET | `/budget` | Все бюджеты пользователя |
| PUT | `/budget/:id` | Обновить лимит |
| DELETE | `/budget/:id` | Удалить бюджет |

### Отчёты
| Метод | URL | Описание |
|-------|-----|----------|
| GET | `/reports/summary` | Сводка за период |
| GET | `/reports/by-category` | Расходы по категориям |
| GET | `/reports/trend` | Тренд по месяцам |
| GET | `/reports/forecast` | Прогноз на следующий месяц |
| GET | `/reports/daily` | Расходы по дням для графика |

> Все эндпоинты кроме `/auth/*` требуют `Authorization: Bearer <token>`

## 💡 Примеры запросов

**Регистрация**
```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret123","name":"Саша"}'
```

**Создать транзакцию**
```bash
curl -X POST http://localhost:8080/transactions \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"amount":5000,"type":"expense","category":"еда","date":"2026-05-13"}'
```

**Создать бюджет**
```bash
curl -X POST http://localhost:8080/budget \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"category":"еда","limit_amount":20000,"period":"monthly"}'
```

**Получить отчёт за период**
```bash
curl "http://localhost:8080/reports/summary?from=2026-05-01&to=2026-05-31" \
  -H "Authorization: Bearer <TOKEN>"
```

**История транзакций с фильтром**
```bash
curl "http://localhost:8080/transactions?category=еда&type=expense" \
  -H "Authorization: Bearer <TOKEN>"
```

## 🧪 Тесты

```bash
# Запустить все тесты
go test ./... -v

# С покрытием
go test ./... -cover
```

## 🔄 CI/CD

При каждом push в `main` GitHub Actions автоматически:
1. Устанавливает зависимости
2. Запускает все тесты
3. Проверяет линтером

## 📝 Переменные окружения

| Переменная | Сервис | Описание |
|---|---|---|
| `GRPC_PORT` | transaction, budget, report | Порт gRPC сервера |
| `DB_HOST` | transaction, budget | Хост PostgreSQL |
| `DB_NAME` | transaction, budget | Имя БД |
| `DB_USER` | transaction, budget | Пользователь БД |
| `DB_PASSWORD` | transaction, budget | Пароль БД |
| `TRANSACTION_ADDR` | report, gateway | Адрес Transaction Service |
| `BUDGET_ADDR` | gateway | Адрес Budget Service |
| `REPORT_ADDR` | gateway | Адрес Report Service |
| `JWT_SECRET` | gateway | Секрет для JWT |
| `REDIS_ADDR` | report | Адрес Redis |
| `SERVER_PORT` | gateway | Порт REST API |

## 🆚 Почему gRPC внутри, REST снаружи

| | REST (Gateway → User) | gRPC (Service → Service) |
|---|---|---|
| Читаемость | человекочитаемый JSON | бинарный protobuf |
| Скорость | медленнее | в 5-10 раз быстрее |
| Браузер | да | нет |
| Типизация | нет | строгая |
| Стриминг | нет | есть |

REST используем для публичного API — браузеры и мобильные приложения его понимают. gRPC для внутреннего общения — скорость и типизация важнее читаемости.