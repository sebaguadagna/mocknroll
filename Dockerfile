FROM golang:1.25.0-alpine3.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mocknroll . && mkdir /data

FROM scratch
COPY --from=build /mocknroll /mocknroll
COPY --from=build --chown=65532:65532 /data /data
USER 65532:65532
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/mocknroll"]
