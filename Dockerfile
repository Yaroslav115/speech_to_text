FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN go build -o /out/server ./cmd/server

FROM alpine:3.20

RUN adduser -D -H appuser
USER appuser

COPY --from=build /out/server /server
EXPOSE 8080

ENTRYPOINT ["/server"]
