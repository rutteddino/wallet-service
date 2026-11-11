.PHONY: up down restart build logs psql test test-race load lint clean

# Поднять всю систему (app + postgres) с пересборкой
up:
	docker compose up -d --build

# Остановить и удалить контейнеры и volume с данными
down:
	docker compose down -v

# Перезапуск приложения
restart:
	docker compose restart app

# Собрать бинарник локально
build:
	go build -o bin/server ./cmd/api

# Логи приложения
logs:
	docker compose logs -f app

# Консоль Postgres в контейнере
psql:
	docker exec -it wallet-postgres psql -U postgres -d wallet

# Unit + интеграционные тесты (нужен запущенный Docker)
test:
	go test ./... -count=1 -v

# Тесты с race detector в контейнере
# (на Windows нет gcc, поэтому гоняем в golang-образе с пробросом docker-сокета)
test-race:
	docker run --rm \
		-v //var/run/docker.sock:/var/run/docker.sock \
		-v "$(CURDIR):/src" \
		-v wallet-gomod-cache:/go/pkg/mod \
		-v wallet-gobuild-cache:/root/.cache/go-build \
		-w /src golang:1.27-bookworm \
		go test ./... -race -count=1

# Нагрузочный тест k6 (нужна запущенная система: make up)
load:
	docker run --rm \
		-v "$(CURDIR)/scripts:/scripts" \
		-e BASE_URL=http://host.docker.internal:8080 \
		grafana/k6:latest run /scripts/load-test.js

# Линтер (требует установленный golangci-lint)
lint:
	golangci-lint run

# Удалить локальный бинарник
clean:
	rm -rf bin/
