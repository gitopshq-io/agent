FROM golang:1.26.1-alpine AS build

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/gitopshq-agent ./cmd/gitopshq-agent

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/gitopshq-agent /usr/local/bin/gitopshq-agent

ENTRYPOINT ["/usr/local/bin/gitopshq-agent"]
