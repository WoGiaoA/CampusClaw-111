ARG IMAGE_REGISTRY=public.ecr.aws/docker/library
FROM ${IMAGE_REGISTRY}/golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY internal/mcpwire ./internal/mcpwire
COPY mcp-server ./mcp-server
RUN CGO_ENABLED=0 go build -trimpath -o /out/campus-mcp ./mcp-server
FROM ${IMAGE_REGISTRY}/alpine:3.23
COPY --from=build /out/campus-mcp /usr/local/bin/campus-mcp
USER 10001:10001
ENTRYPOINT ["campus-mcp"]
