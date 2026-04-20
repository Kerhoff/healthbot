FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /healthbot ./cmd/bot

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /healthbot /healthbot
EXPOSE 8080
ENTRYPOINT ["/healthbot"]
