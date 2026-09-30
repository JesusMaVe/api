# syntax=docker/dockerfile:1
ARG GO_VERSION=1.27.1
ARG ALPINE_VERSION=3.24

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/api /api
# uid/gid del usuario nonroot de distroless (numérico: no depende de /etc/passwd)
USER 65532:65532
HEALTHCHECK --interval=10s --timeout=4s --start-period=15s --start-interval=1s \
  CMD ["/api", "-healthcheck"]
ENTRYPOINT ["/api"]
