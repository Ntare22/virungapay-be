# Project virungapay-be

One Paragraph of project description goes here

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

## MakeFile

Run build make command with tests
```bash
make all
```

Build the application
```bash
make build
```

Run the application
```bash
make run
```
Create DB container
```bash
make docker-run
```

Shutdown DB Container
```bash
make docker-down
```

DB Integrations Test:
```bash
make itest
```

Live reload the application:
```bash
make watch
```

Apply SQL migrations:
```bash
make migrate-up
```

Roll back one migration:
```bash
make migrate-down
```

Lint:
```bash
make lint
```

Regenerate the OpenAPI spec after changing route comments:
```bash
make docs
```

Swagger UI is served at `http://localhost:8080/docs/index.html` when `APP_ENV` is not `production`. Copy `.env.example` to `.env` before `make run`. The API reads `DATABASE_URL`.

Run the test suite:
```bash
make test
```

Clean up binary from the last build:
```bash
make clean
```
