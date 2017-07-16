FROM golang

RUN apt-get update
RUN apt-get install -y wget curl rtmpdump xz-utils
RUN sh -c "cd /tmp && wget http://johnvansickle.com/ffmpeg/releases/ffmpeg-release-64bit-static.tar.xz && ls && tar Jxvf *.tar.xz && cp ./ffmpeg*/ffmpeg /usr/local/bin"
RUN mkdir -p $GOPATH/src/github.com/cs3238-tsuzu/agqrrecorder
COPY . $GOPATH/src/github.com/cs3238-tsuzu/agqrrecorder

RUN go get github.com/cs3238-tsuzu/agqrrecorder

EXPOSE 80
ENTRYPOINT agqrrecorder
