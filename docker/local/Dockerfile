FROM golang:1.21-bullseye AS builder
WORKDIR /app
COPY . .
COPY ./docker/local/config.toml .

RUN go build -v -o nitro .
RUN go build -v -o proxy ./cmd/start-payment-proxy

FROM debian:bullseye-slim
RUN apt-get update
RUN apt-get install -y ca-certificates jq netcat
RUN rm -rf /var/lib/apt/lists/*
WORKDIR /app/
COPY --from=builder /app/nitro .
COPY --from=builder /app/proxy .
COPY --from=builder /app/config.toml .

EXPOSE 3005 4005 5005

CMD ["./nitro", "--config", "./config.toml"]
