FROM golang:1.12
# Build Image

COPY . /go/src/github.com/spkane/nr-tesla
WORKDIR /go/src/github.com/spkane/nr-tesla

ENV CGO_ENABLED=0
ENV GOOS=linux

RUN go build .

FROM alpine:latest
# Deploy Image

RUN apk --no-cache add ca-certificates

WORKDIR /
COPY --from=0 /go/src/github.com/spkane/nr-tesla/nr-tesla .

CMD ["/nr-tesla"]

