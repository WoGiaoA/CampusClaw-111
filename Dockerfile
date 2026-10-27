ARG IMAGE_REGISTRY=public.ecr.aws/docker/library
FROM ${IMAGE_REGISTRY}/golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -o /out/campusclaw ./cmd/api
FROM ${IMAGE_REGISTRY}/alpine:3.23
RUN addgroup -g 10001 campus && adduser -D -u 10001 -G campus campus && mkdir /uploads && chown campus:campus /uploads
COPY --from=build /out/campusclaw /usr/local/bin/campusclaw
USER campus
EXPOSE 8080
ENTRYPOINT ["campusclaw"]
