FROM golang

COPY . $GOPATH/src/github.com/cs3238-tsuzu/agqrrecorder
RUN apt-get update && \
    apt-get install -y wget curl rtmpdump xz-utils && \
    sh -c "cd /tmp && wget http://johnvansickle.com/ffmpeg/releases/ffmpeg-release-64bit-static.tar.xz && ls && tar Jxvf *.tar.xz && cp ./ffmpeg*/ffmpeg /usr/local/bin && rm -r /tmp/*.tar.gz" && \
    mkdir -p $GOPATH/src/github.com/cs3238-tsuzu/agqrrecorder && \
    go get github.com/cs3238-tsuzu/agqrrecorder

EXPOSE 80
ENTRYPOINT agqrrecorder
