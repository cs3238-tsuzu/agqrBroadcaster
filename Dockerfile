FROM golang:1.12 as build

RUN CGO_ENABLED=0 go get github.com/cs3238-tsuzu/agqrBroadcaster

FROM ubuntu:18.04

COPY --from=build /go/bin/agqrBroadcaster /bin/
RUN apt-get update && \
    apt-get install -y rtmpdump ffmpeg

EXPOSE 80
ENTRYPOINT agqrBroadcaster
