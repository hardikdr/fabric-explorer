FROM node:24-alpine AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN --mount=type=cache,target=/root/.cache/go-build GOMAXPROCS=2 CGO_ENABLED=0 go build -p=2 -trimpath -ldflags="-s -w" -o /fabric-explorer ./cmd/fabric-explorer

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /fabric-explorer /app/fabric-explorer
COPY --from=web /web/dist /app/web/dist
COPY testdata/kubernetes/ /app/testdata/kubernetes/
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/fabric-explorer"]
CMD ["--mode=fixture", "--listen=0.0.0.0:8080", "--web-dir=/app/web/dist"]
