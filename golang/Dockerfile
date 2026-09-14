FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 65532 appuser

COPY --from=build /out/api /api

ENV PORT=8080
EXPOSE 8080

USER appuser
ENTRYPOINT ["/api"]
