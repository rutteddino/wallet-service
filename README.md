# Wallet Service

[![CI](https://github.com/rutteddino/wallet-service/actions/workflows/ci.yml/badge.svg)](https://github.com/rutteddino/wallet-service/actions/workflows/ci.yml)

REST-сервис для управления балансом кошельков. Тестовое задание.

## Стек

- **Go 1.27** (chi, pgx/v5, slog)
- **PostgreSQL 16** (миграции на golang-migrate)
- **Docker / docker-compose**
- Тесты: stdlib и **testcontainers** (интеграционные на реальном Postgres)

## Быстрый старт

```bash
cp config.env.example config.env   # файл с настройками не хранится в git
docker compose up --build          # или: make up
```

Сервис поднимется на `http://localhost:8080`, миграции применятся автоматически при старте. У обоих контейнеров есть healthcheck: compose ждёт готовности Postgres до старта приложения, а приложение становится healthy после поднятия HTTP-сервера.

Остановка с очисткой данных:

```bash
docker compose down -v         # или: make down
```

## API

### Изменить баланс

```bash
curl -X POST http://localhost:8080/api/v1/wallet \
  -H "Content-Type: application/json" \
  -d '{"walletId":"11111111-1111-1111-1111-111111111111","operationType":"DEPOSIT","amount":1000}'
```

- `operationType`: `DEPOSIT` или `WITHDRAW`
- `amount`: целое, в минорных единицах (копейках), `> 0`
- Кошелёк **создаётся автоматически** первым депозитом
- Ответ: `{"walletId":"...","balance":1000}`

### Идемпотентность

Заголовок `Idempotency-Key` делает ретраи безопасными: повторный запрос с тем же ключом вернёт текущий баланс, **не изменив его повторно**:

```bash
curl -X POST http://localhost:8080/api/v1/wallet \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: my-operation-123" \
  -d '{"walletId":"11111111-1111-1111-1111-111111111111","operationType":"DEPOSIT","amount":1000}'
```

### Получить баланс

```bash
curl http://localhost:8080/api/v1/wallets/11111111-1111-1111-1111-111111111111
```

### Проверка живости

```bash
curl http://localhost:8080/health    # {"status":"ok"}
```

Используется healthcheck-ом контейнера `app` в compose и пробой готовности в CI.

### Коды ответов

| Код | Когда |
|---|---|
| 200 | операция применена или баланс получен (включая идемпотентный ретрай) |
| 400 | битый JSON, невалидный UUID, тип операции или сумма |
| 404 | кошелёк не найден |
| 422 | недостаточно средств для WITHDRAW |
| 500 | только непредвиденные ошибки (штатные сценарии 5xx не возвращают) |

## Конфигурация

Все настройки читаются из `config.env` (см. `config.env.example`), переменные окружения имеют приоритет. Сам файл в gitignore: в CI он восстанавливается из секрета `CONFIG_ENV`, а если секрета нет, берётся пример.

| Переменная | По умолчанию | Описание |
|---|---|---|
| `SERVER_PORT` | `8080` | порт HTTP-сервера |
| `DB_HOST` / `DB_PORT` | `localhost` / `5432` | адрес PostgreSQL |
| `DB_USER` / `DB_PASSWORD` | `postgres` / без дефолта | креды (`DB_PASSWORD` обязателен) |
| `DB_NAME` | `wallet` | имя базы |
| `DB_MAX_CONNS` | `25` | размер пула соединений |
| `MIGRATIONS_PATH` | `file://migrations` | путь к миграциям |

## Архитектура

```
cmd/api/main.go              точка входа: конфиг, миграции, wiring, graceful shutdown
internal/
  config/                    загрузка и валидация config.env
  domain/                    сущности, типы операций, доменные ошибки
  repository/                порт (интерфейс) и postgres-реализация
  service/                   бизнес-логика, валидация, генерация idempotency key
  handler/                   HTTP-слой: роутер, хендлеры, middleware, маппинг ошибок
migrations/                  миграции golang-migrate
```

Ключевые решения:

- **Баланс хранится в `BIGINT`** в минорных единицах с `CHECK (balance >= 0)` на уровне БД.
- Слои изолированы: хендлер не знает про SQL, репозиторий не знает про HTTP. Ошибки между слоями доменные (`ErrWalletNotFound`, `ErrInsufficientFunds`).
- Graceful shutdown: SIGTERM, перестаём принимать соединения, даём до 10 секунд на завершение активных запросов.

## Конкурентность (1000 RPS по одному кошельку)

Изменение баланса выполняется **одним атомарным SQL-запросом** в транзакции вместе с записью об операции:

```sql
-- DEPOSIT (с автосозданием кошелька)
INSERT INTO wallets (id, balance) VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE
SET balance = wallets.balance + EXCLUDED.balance, updated_at = now()
RETURNING balance;

-- WITHDRAW (уход в минус отсекается условием в WHERE)
UPDATE wallets SET balance = balance - $2, updated_at = now()
WHERE id = $1 AND balance >= $2
RETURNING balance;
```

- Postgres берёт **короткую row-level блокировку** на строку кошелька: конкурентные операции сериализуются самой БД, гонок и lost update нет.
- Проверка достаточности средств выполняется **в самом UPDATE**. Вариант "прочитал, проверил, записал" давал бы race condition.
- Транзакции короткие (2 стейтмента), лочится одна строка, дедлоки исключены.
- **Идемпотентность**: уникальный `idempotency_key` в таблице операций. Ретрай откатывает своё изменение и возвращает текущий баланс с кодом 200.
- Итог: штатные ответы только 200 и 4xx, 5xx при нагрузке не возникают.

Корректность доказана конкурентным тестом: **1000 горутин** (500 депозитов и 500 выводов) по одному кошельку, итоговый баланс сходится точно, ни одной ошибки.

## Нагрузочное тестирование (k6)

Скрипт `scripts/load-test.js` гоняет 1000 RPS по одному кошельку 30 секунд:

```bash
make up     # система должна быть запущена
make load
```

Замер на Docker Desktop (Windows):

| Метрика | Значение |
|---|---|
| Запросов всего | 12718 |
| Ошибки 5xx | **0** |
| Ошибки 4xx | **0** (все ответы 200) |
| Устойчивый RPS | ~414 |
| Latency avg / p95 | 706 / 848 мс |

Про устойчивый RPS: пропускная способность по **одному** кошельку физически ограничена сериализацией на row-level блокировке (каждая транзакция ждёт предыдущую на той же строке). Система ведёт себя правильно: лишние запросы **ждут в очереди**, а не отваливаются с 5xx. По разным кошелькам операции не блокируют друг друга и масштабируются линейно.

## Тесты

```bash
go test ./... -count=1          # unit и интеграционные (нужен запущенный Docker)
make test                       # то же самое
make test-race                  # прогон с -race в контейнере golang:bookworm
```

- **Unit**: сервис (валидация, генерация idempotency key) и хендлер (коды ответов, маппинг ошибок) с моком репозитория.
- **Integration**: testcontainers поднимает реальный Postgres, накатываются настоящие миграции. Проверяются все операции, идемпотентность и конкурентный сценарий.

Покрытие (`go test -cover ./...`): handler **100%**, service **100%**, repository/postgres **49%** (непокрытые ветки это ретраи подключения и ошибки инфраструктуры).

## Команды Makefile

`make up` · `make down` · `make test` · `make test-race` · `make load` · `make logs` · `make psql` · `make build`
