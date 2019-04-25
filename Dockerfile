FROM golang:1.12.4-alpine3.9 AS builder

RUN apk add --update --no-cache make bash git openssh-client build-base musl-dev curl wget

ADD . /src/app

WORKDIR /src/app

RUN mkdir ./bin && \
    go build -o ./bin/postmanq -a cmd/postmanq.go && \
    go build -o ./bin/pmq-grep -a cmd/pmq-grep.go && \
    go build -o ./bin/pmq-publish -a cmd/pmq-publish.go && \
    go build -o ./bin/pmq-report -a cmd/pmq-report.go

FROM alpine:3.9

COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /usr/local/go/lib/time/zoneinfo.zip
COPY --from=builder /src/app/bin/postmanq /bin/postmanq
COPY --from=builder /src/app/bin/pmq-grep /bin/pmq-grep
COPY --from=builder /src/app/bin/pmq-publish /bin/pmq-publish
COPY --from=builder /src/app/bin/pmq-report /bin/pmq-report


ENTRYPOINT ["postmanq"]
CMD ["-f", "/etc/postmaq.yaml"]