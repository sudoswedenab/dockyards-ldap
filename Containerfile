FROM docker.io/library/golang:1.25.0 AS builder
COPY . /src
WORKDIR /src
ENV CGO_ENABLED=0
RUN go build -o dockyards-ldap --ldflags="-s -w"

FROM scratch
COPY --from=builder /src/dockyards-ldap /usr/bin/dockyards-ldap
ENTRYPOINT ["/usr/bin/dockyards-ldap"]
