FROM golang:1.16-alpine3.14 AS builder

RUN apk add --update --no-cache make bash git openssh-client build-base musl-dev curl wget

ADD . /src/app

WORKDIR /src/app

RUN mkdir ./bin && \
    CGO_ENABLED=0 go build -i -ldflags '-d -s -w' -o ./bin/postmanq -a cmd/postmanq/main.go && \
    CGO_ENABLED=0 go build -i -ldflags '-d -s -w' -o ./bin/pmq-grep -a cmd/tools/pmq-grep/main.go && \
    CGO_ENABLED=0 go build -i -ldflags '-d -s -w' -o ./bin/pmq-publish -a cmd/tools/pmq-publish/main.go && \
    CGO_ENABLED=0 go build -i -ldflags '-d -s -w' -o ./bin/pmq-report -a cmd/tools/pmq-report/main.go

RUN addgroup -g 1001 postmanq && \
    adduser -S -u 1001 -G postmanq postmanq && \
    chown -R 1001:1001 /src/app

FROM alpine:3.14

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /usr/local/go/lib/time/zoneinfo.zip
COPY --from=builder /src/app/bin/postmanq /postmanq
COPY --from=builder /src/app/bin/pmq-grep /pmq-grep
COPY --from=builder /src/app/bin/pmq-publish /pmq-publish
COPY --from=builder /src/app/bin/pmq-report /pmq-report

USER postmanq:postmanq

ENTRYPOINT ["/postmanq"]
CMD ["-f", "/postmaq.yaml"]
