FROM golang:1.12.4-alpine3.9 AS builder

RUN apk add --update --no-cache make bash git openssh-client build-base musl-dev curl wget

ADD . /src/app

WORKDIR /src/app

RUN mkdir ./bin && \
    CGO_ENABLED=0 go build -i -ldflags '-d -s -w' -o ./bin/postmanq -a cmd/postmanq.go && \
    CGO_ENABLED=0 go build -i -ldflags '-d -s -w' -o ./bin/pmq-grep -a cmd/pmq-grep.go && \
    CGO_ENABLED=0 go build -i -ldflags '-d -s -w' -o ./bin/pmq-publish -a cmd/pmq-publish.go && \
    CGO_ENABLED=0 go build -i -ldflags '-d -s -w' -o ./bin/pmq-report -a cmd/pmq-report.go

FROM scratch

COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /usr/local/go/lib/time/zoneinfo.zip
COPY --from=builder /src/app/bin/postmanq /postmanq
COPY --from=builder /src/app/bin/pmq-grep /pmq-grep
COPY --from=builder /src/app/bin/pmq-publish /pmq-publish
COPY --from=builder /src/app/bin/pmq-report /pmq-report


ENTRYPOINT ["/postmanq"]
CMD ["-f", "/postmaq.yaml"]