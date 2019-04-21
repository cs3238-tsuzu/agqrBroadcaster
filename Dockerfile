FROM golang

COPY . $GOPATH/src/github.com/cs3238-tsuzu/agqrrecorder
RUN apt-get update && \
    apt-get install -y wget curl rtmpdump xz-utils ffmpeg && \
    mkdir -p $GOPATH/src/github.com/cs3238-tsuzu/agqrrecorder && \
    go get github.com/cs3238-tsuzu/agqrrecorder

EXPOSE 80
ENTRYPOINT agqrrecorder
