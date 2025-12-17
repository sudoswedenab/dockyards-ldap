FROM docker.io/library/golang:1.25.0 AS builder
COPY . /src
WORKDIR /src
ENV CGO_ENABLED=0
RUN go build -o dockyards-keycloak --ldflags="-s -w"

FROM scratch
COPY --from=builder /src/dockyards-keycloak /usr/bin/dockyards-keycloak
ENTRYPOINT ["/usr/bin/dockyards-keycloak"]
