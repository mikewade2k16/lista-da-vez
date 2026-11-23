FROM --platform=$BUILDPLATFORM golang:1.26.5-bookworm AS build

ARG TARGETOS=linux
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Cache do compilador Go persistido pelo BuildKit entre builds: so os pacotes
# alterados recompilam. Nao entra na imagem final (o runtime so recebe os binarios).
RUN --mount=type=cache,id=omni-go-build,target=/root/.cache/go-build \
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/api ./cmd/api \
  && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/migrate ./cmd/migrate

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl ffmpeg tzdata tesseract-ocr tesseract-ocr-por poppler-utils \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/migrate /usr/local/bin/migrate

EXPOSE 8080

CMD ["sh", "-c", "migrate up && migrate bootstrap-erp-store && api"]
