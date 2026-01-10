FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o lucene-shard-analyzer .

FROM alpine:3.19

ARG GIT_SHA=unknown

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/lucene-shard-analyzer .

ENV GIT_SHA=${GIT_SHA}

EXPOSE 8080

CMD ["./lucene-shard-analyzer", "-port", "8080"]
