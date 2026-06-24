.PHONY: build run dev-up dev-down seed test test-integration lint

build:
	go build -o bin/controlplane ./cmd/controlplane

run: build
	./bin/controlplane

# Start local infra (Postgres + Mongo + Redis)
dev-up:
	docker compose up -d --wait
	@echo "Postgres, Mongo and Redis are up."
	@echo "Now run 'make seed' (after the control plane is running) or start the controlplane with make run."

dev-down:
	docker compose down -v

# Seed a demo tenant + backend + policy + token
# Requires the control plane to be running on :8080 and NANCE_ADMIN_TOKEN (or none in dev)
seed:
	@echo "Creating demo tenant..."
	curl -s -X POST http://localhost:8080/api/v1/tenants \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $${NANCE_ADMIN_TOKEN:-dev}" \
		-d '{"id":"demo","name":"Demo Tenant"}' | cat

	@echo "\nSetting backend (pointing at the compose mongo)..."
	curl -s -X POST http://localhost:8080/api/v1/tenants/demo/backend \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $${NANCE_ADMIN_TOKEN:-dev}" \
		-d '{"uri":"mongodb://root:example@localhost:27017/?authSource=admin"}' | cat

	@echo "\nTesting the backend connection..."
	curl -s -X POST http://localhost:8080/api/v1/tenants/demo/backend/test \
		-H "Authorization: Bearer $${NANCE_ADMIN_TOKEN:-dev}" | cat

	@echo "\nSetting a sample cache policy..."
	curl -s -X PUT http://localhost:8080/api/v1/tenants/demo/policy/collections/mydb.users \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $${NANCE_ADMIN_TOKEN:-dev}" \
		-d '{"enabled":true,"ttlSeconds":300,"maxResultBytes":1048576}' | cat

	@echo "\nIssuing a data-plane token (copy the rawToken!):"
	curl -s -X POST http://localhost:8080/api/v1/tenants/demo/tokens \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $${NANCE_ADMIN_TOKEN:-dev}" \
		-d '{"description":"local dev token"}' | cat

	@echo "\n\nDone. Use the rawToken with mongodb+nance://demo:<rawToken>@127.0.0.1:27017/mydb (in Phase 1+)."

test:
	go test ./...

test-integration:
	go test -tags=integration -count=1 ./...

lint:
	go vet ./...
	gofmt -l .
