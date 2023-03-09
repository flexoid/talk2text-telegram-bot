# syntax=docker/dockerfile:1

## Build
FROM golang:1.20-alpine3.17 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /talk2text-telegram-bot

## Deploy
FROM alpine:3.17

RUN apk --no-cache add ca-certificates ffmpeg

COPY --from=builder /talk2text-telegram-bot /talk2text-telegram-bot

CMD ["/talk2text-telegram-bot"]
