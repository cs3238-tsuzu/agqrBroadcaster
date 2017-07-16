package main

import (
	"os/exec"
	"os"
	"net/http"
	"sync/atomic"
	"sync"
	"io"
	"github.com/Sirupsen/logrus"
)

var idx int64
var mux sync.Mutex
var clients map[int64]chan []byte

func AddClient(ch chan []byte) int64 {
	n := atomic.AddInt64(&idx, 1)

	mux.Lock()
	defer mux.Unlock()

	clients[n] = ch

	return n
}

func DeleteClient(i int64) {
	mux.Lock()
	defer mux.Unlock()

	delete(clients, i)
}

const HTML = `<!DOCTYPE html>
<html>
<head>
<title>Audio Transporter</title>
</head>
<body>
<h1>Audio</h1>
<audio id="stream" src="/audio" preload="none" controls>
<p>Your browser does not support the <code>audio</code> element.</p>
<script>
$(function(){
//	var player = document.getElementById('stream');

//	player.play();
});
</script>
</audio>
</body>
</html>
`

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	clients = make(map[int64]chan []byte)

	init := func() (*exec.Cmd, io.ReadCloser, error) {
		cmd := exec.Command("sh", "-c", "rtmpdump --live -r rtmp://fms-base1.mitene.ad.jp/agqr/aandg22 -o - 2>/dev/null | ffmpeg -i pipe:0 -acodec mp3 -f mp3 pipe:1 2> /dev/null")
		//cmd := exec.Command("sh", "-c", "ffmpeg -f avfoundation -i \":0\" -acodec mp3 -f mp3 - 2>/dev/null")

		cmd.Env = append(os.Environ(), "DYLD_LIBRARY_PATH=/usr/local/Cellar/x264/r2533/lib")
		cmd.Stderr = os.Stderr
		reader, err := cmd.StdoutPipe()

		if err != nil {
			return nil, nil, err
		}

		err = cmd.Start()

		if err != nil {
			reader.Close()
			return nil, nil, err
		}

		return cmd, reader, nil
	}

	cmd, reader, err := init()

	if err != nil {
		panic(err)
	}

	go func() {
		http.ListenAndServe(":80", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/" {

				rw.Write([]byte(HTML))
				return
			}
			if req.URL.Path != "/audio" {
				rw.WriteHeader(http.StatusNotFound)

				return
			}

			logrus.Debug("Accepted")
			rw.Header().Set("Content-Type", "audio/mpeg")

			rw.WriteHeader(http.StatusOK)

			req.Body.Close()
			ch := make(chan []byte, 1000)
			i := AddClient(ch)
			logrus.WithField("idx", i).Debug("Connected")
			defer close(ch)
			defer DeleteClient(i)

			notifier := rw.(http.CloseNotifier).CloseNotify()
			for {
				select {
				case <-notifier:
					logrus.WithField("idx", i).Debug("Disconnected")
					return

				case b := <-ch:
					rw.Write(b)
				}
			}
		}))
	}()

	for {
		buf := make([]byte, 4096)
		len, err := reader.Read(buf)

		if err != nil {
			logrus.WithError(err).Error("Reading from stdout error")

			cmd.Process.Kill()
			cmd.Wait()
			reader.Close()
			cmd, reader, err = init()

			if err != nil {
				panic(err)
			}
		}

		func() {
			b := buf[:len]
			mux.Lock()
			defer mux.Unlock()

			for _, v := range clients {
				select {
				case v <- b:
				default:
				}
			}
		}()
	}
}
