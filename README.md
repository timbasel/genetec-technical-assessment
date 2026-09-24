# Book-keeping service

A Go HTTP service for books and their change history.

## Run

Requires Go 1.27. From the repository root:

```sh
go run ./cmd/server
```

The server listens on `127.0.0.1:8080` and stores data in `data/books.db` by default. Set `BOOKS_HTTP_ADDRESS` or `BOOKS_DATABASE_PATH` to change them. Explore the API at http://127.0.0.1:8080/docs/. The OpenAPI spec is at http://127.0.0.1:8080/openapi.yaml.

## Test

```sh
go test ./...
```

## Design Decisions

SQLite stores a book and its field-level history entries in one transaction. History rows are append-only and retain old and new JSON values, a description written at the time of the change, and a Unix-nanosecond timestamp.

PUT replaces the book's fields and requires its current `version`. The storage layer checks the version during the update; stale writes return HTTP 409. An update that changes nothing creates no history entries.

History supports filtering, offset pagination, and ordering by timestamp then entry ID. Offset pagination is simple, but pages can shift when new changes arrive between requests. The OpenAPI spec is in `internal/api/openapi.yaml`; the browser UI uses local bundled assets.
