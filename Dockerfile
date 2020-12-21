# syntax=docker/dockerfile:experimental
FROM golang:1.14 as builder

ARG GO_VERSION="GoVersion"
ARG GIT_COMMIT="git_commit"
ARG MoiraVersion="MoiraVersion"

WORKDIR /go/src/github.com/moira-alert/moira

RUN apt-get update && apt-get install -y mercurial

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . /go/src/github.com/moira-alert/moira/

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN for bin in "api" "checker" "filter"; do \
    go build -a -installsuffix cgo -ldflags "-X main.MoiraVersion=${MoiraVersion} -X main.GoVersion=${GO_VERSION} -X main.GitCommit=${GIT_COMMIT}" -o /usr/local/moira/bin/$bin go.avito.ru/DO/moira/cmd/$bin; done

FROM debian-minbase:latest
COPY --from=builder /usr/local/moira/bin/* /usr/local/bin/

RUN echo "Europe/Moscow" > /etc/timezone
RUN dpkg-reconfigure -f noninteractive tzdata
