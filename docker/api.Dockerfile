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
    -o build/api \
    go.avito.ru/DO/moira/cmd/api


FROM registry.k.avito.ru/avito/debian-minbase:latest

RUN apt-get update && apt-get install -y ca-certificates netcat ngrep

COPY pkg/api/api.yml /etc/moira/api.yml
COPY pkg/api/web.json /etc/moira/web.json
COPY --from=builder /go/src/go.avito.ru/DO/moira/build/api /usr/bin/api

EXPOSE 8081 8081

ENTRYPOINT [ "/usr/bin/api" ]
