FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/finops-guard ./cmd/finops-guard

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /workspace

COPY --from=builder /out/finops-guard /usr/local/bin/finops-guard
COPY pricing.json /workspace/pricing.json

ENTRYPOINT ["finops-guard"]
