# syntax=docker/dockerfile:experimental
FROM golang:1.14 as builder

WORKDIR /go/src/go.avito.ru/DO/moira

RUN apt-get update && apt-get install -y mercurial

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . /go/src/go.avito.ru/DO/moira/

ARG GO_VERSION="1.14"
ARG GIT_COMMIT="git_Commit"
ARG MoiraVersion="MoiraVersion"

RUN CGO_ENABLED=0 GOOS=linux \
    go build -a -installsuffix cgo \
    -ldflags "-X main.MoiraVersion=${MoiraVersion} -X main.GoVersion=${GO_VERSION} -X main.GitCommit=${GIT_COMMIT}" \
    -o build/checker \
    go.avito.ru/DO/moira/cmd/checker


FROM registry.k.avito.ru/avito/debian-minbase:latest

RUN apt-get update && apt-get install -y ca-certificates netcat ngrep

COPY pkg/checker/checker.yml /etc/moira/checker.yml
COPY --from=builder /go/src/go.avito.ru/DO/moira/build/* /usr/bin/checker

ENTRYPOINT [ "/usr/bin/checker" ]
