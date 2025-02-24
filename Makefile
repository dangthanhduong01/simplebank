DB_URL=postgresql://postgres:12345@localhost:5432/simplebank?sslmode=disable

network:
	docker network create bank-network

postgres:
	docker run --name postgres --network bank-network -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=secret -d postgres:14-alpine

mysql:
	docker run --name mysql8 -p 3306:3306  -e MYSQL_ROOT_PASSWORD=secret -d mysql:8

createdb:
	docker exec -it postgres createdb --username=root --owner=root simple_bank

dropdb:
	docker exec -it postgres dropdb simple_bank

migrateup:
	../../../go/bin/migrate -path db/migration -database "$(DB_URL)" -verbose up

migrateup1:
	../../../go/bin/migrate -path db/migration -database "$(DB_URL)" -verbose up 1

migratedown:
	../../../go/bin/migrate -path db/migration -database "$(DB_URL)" -verbose down

migratedown1:
	../../../go/bin/migrate -path db/migration -database "$(DB_URL)" -verbose down 1

new_migration:
	migrate create -ext sql -dir db/migration -seq ${name}

db_schema:
	dbml2sql --postgres -o doc/schema.sql doc/db.dbml

sqlc:
	../../../go/bin/sqlc generate

mock:
	../../../go/bin/mockgen -package mockdb -destination db/mock/store.go github.com/dangthanhduong01/simplebank/db/sqlc Store

server:
	go run main.go

proto:
	rm -f pb/*.go
	rm -f doc/swagger/*.swagger.json
	protoc --proto_path=proto --go_out=pb --go_opt=paths=source_relative \
	--go-grpc_out=pb --go-grpc_opt=paths=source_relative \
	--grpc-gateway_out=pb --grpc-gateway_opt=paths=source_relative \
	--openapiv2_out=doc/swagger --openapiv2_opt=allow_merge=true,merge_file_name=simple_bank \
    proto/*.proto
	../../../go/bin/statik -src=./doc/swagger -dest=./doc

evans:
	../../../go/bin/evans --host localhost --port 9090 -r repl

redis:
	docker run --name redis -p 6379:6379 -d redis:7-alpine

.PHONY: network postgres createdb dropdb migrateup migratedown migrateup1 migratedown1 new_migration sqlc server mock proto evans redis