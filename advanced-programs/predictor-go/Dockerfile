FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/predictorctl ./cmd/predictorctl \
 && go build -o /out/instrument-orchestrator ./cmd/instrument-orchestrator \
 && go build -o /out/predictor-writer ./cmd/predictor-writer \
 && go build -o /out/predictor-exchange ./cmd/predictor-exchange

FROM alpine:3.20

WORKDIR /app
COPY --from=build /out/ /app/
COPY configs/ /app/configs/

ENTRYPOINT ["/app/predictorctl"]
