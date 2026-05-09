ifneq (,$(wildcard ./.env))
	include .env
	export
endif

check-db-url:
	@if [ -z "$(DATABASE_URL)" ]; then echo "DATABASE_URL is not set in .env"; exit 1; fi

gen-grpc-%:
	cd contracts; protoc --go_out=./gen/$*_pb --go_opt=paths=source_relative \
                  --go-grpc_out=./gen/$*_pb --go-grpc_opt=paths=source_relative \
                  $*.proto

gen-migration-%:
	@if [ -z "$(NAME)" ]; then echo "Enter migration name (NAME=...)"; exit 1; fi
	@echo "Migration generation $(NAME) for service $*..."
	cd $* && goose -dir ./migrations create $(NAME) sql

migrate-%:
	@cd $* && \
	if [ ! -f .env ]; then echo ".env not found in $*"; exit 1; fi; \
	set -a; . ./.env; set +a; \
	if [ -z "$$DATABASE_URL" ]; then echo "DATABASE_URL is not set in $*/.env"; exit 1; fi; \
	goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-down: check-db-url
	goose -dir ./migrations postgres "$(DATABASE_URL)" down

migrate-reset: check-db-url
	goose -dir ./migrations postgres "$(DATABASE_URL)" down-to 0
	goose -dir ./migrations postgres "$(DATABASE_URL)" up

