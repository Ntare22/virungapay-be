FROM golang:1.26.5-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o main ./cmd/api

FROM alpine:3.20.1 AS prod
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /app/main /app/main
COPY --from=build /app/migrations /app/migrations
EXPOSE ${PORT}
CMD ["./main"]


