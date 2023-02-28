.PHONY: start-poc
start-poc:
	@docker compose up -d

.PHONY: cleanup-poc
cleanup-poc:
	@docker compose down  --rmi local --volumes

.PHONY: fmt
fmt:
	@go fmt ./...
