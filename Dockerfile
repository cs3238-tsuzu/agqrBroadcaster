FROM golang:1.12 as build

RUN go get github.com/cs3238-tsuzu/agqrBroadcaster

FROM golang:1.12

COPY --from=build /go/bin/agqrBroadcaster /bin/
RUN apt-get update && \
    apt-get install -y wget curl rtmpdump xz-utils ffmpeg

EXPOSE 80
ENTRYPOINT agqrBroadcaster
