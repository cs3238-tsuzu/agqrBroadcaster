package main

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/Sirupsen/logrus"
	pushbullet "github.com/mitsuse/pushbullet-go"
	"github.com/mitsuse/pushbullet-go/requests"
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

func PushBulletInit(token string) func(title, body string) error {
	pb := pushbullet.New(token)

	return func(title, body string) error {
		note := requests.NewNote()

		note.Body = body
		note.Title = title

		_, err := pb.PostPushesNote(note)

		if err != nil {
			logrus.WithError(err).Error("PushBullet error")
		}

		return err
	}
}

type ServerInfo struct {
	Cryptography string `xml:"cryptography"`
	Protocol     string `xml:"protocol"`
	Server       string `xml:"server"`
	App          string `xml:"app"`
	Stream       string `xml:"stream"`
}

func (i ServerInfo) String() string {
	b, _ := json.Marshal(i)

	return string(b)
}

type Ag struct {
	XMLName xml.Name `xml:"ag"`
	Status  struct {
		Code string `xml:"code"`
	} `xml:"status"`
	ServerList struct {
		ServerInfo []ServerInfo `xml:"serverinfo"`
	} `xml:"serverlist"`
}

func main() {
	pbToken := os.Getenv("PB_TOKEN")

	logrus.SetLevel(logrus.DebugLevel)
	clients = make(map[int64]chan []byte)

	notify := PushBulletInit(pbToken)

	if notify("Notification Test", "Server started") != nil {
		os.Exit(1)
	}

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	rtmpURLGet := func() (*ServerInfo, error) {
		req, err := http.NewRequest("GET", "http://www.uniqueradio.jp/agplayerf/getfmsListHD.php", nil)

		if err != nil {
			return nil, errors.Wrap(err, "NewRequest error")
		}

		resp, err := client.Do(req)

		if err != nil {
			return nil, errors.Wrap(err, "Do error")
		}

		var body []byte

		if resp.Body == nil {
			return nil, errors.New(resp.Status)
		}

		body, err = ioutil.ReadAll(resp.Body)

		if err != nil {
			return nil, errors.Wrap(err, "ReadAll error ("+resp.Status+")")
		}

		if resp.StatusCode != http.StatusOK {
			return nil, errors.New(resp.Status + ": " + string(body))
		}

		var ag Ag

		if err := xml.Unmarshal(body, &ag); err != nil {
			return nil, errors.Wrap(err, "XML unmarshal error"+string(body))
		}

		if len(ag.ServerList.ServerInfo) == 0 {
			return nil, errors.New("Insufficient server: " + string(body))
		}

		serverInfo := ag.ServerList.ServerInfo[0]

		return &serverInfo, nil
	}

	var url ServerInfo
	init := func() (*exec.Cmd, io.ReadCloser, error) {
		newURL, err := rtmpURLGet()

		if err != nil {
			notify("!!!ERROR!!!", err.Error())
		}

		if err == nil && url != *newURL {
			notify("NOTIFICATION", "URL has been changed from "+url.String()+" to "+newURL.String())
		}

		url = *newURL

		logrus.WithField("url", url.String()).Info("URL is ...")

		logrus.WithField("url", "rtmpdump --live -r "+url.Server+" --app "+url.App+" --playpath "+url.Stream+" -o - 2>/dev/null").Info("Command is ...")

		var protocol string
		switch url.Protocol {
		case "rtmp":
			protocol = "0"
		case "rtmpe":
			protocol = "2"
		default:
			e := errors.New("Unsupported Protocol: " + url.Protocol)
			notify("!!!ERROR!!!", e.Error())

			return nil, nil, e
		}

		cmd := exec.Command("sh", "-c", "rtmpdump --live -r "+url.Server+" --protocol "+protocol+" --app "+url.App+" --playpath "+url.Stream+" -o - 2>/dev/null | ffmpeg -i pipe:0 -acodec mp3 -f mp3 pipe:1 2> /dev/null")
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

		for err != nil {
			logrus.WithError(err).Error("Reading from stdout error")

			cmd.Process.Kill()
			cmd.Wait()
			reader.Close()
			cmd, reader, err = init()

			if err != nil {
				logrus.WithError(err).Error("Error")
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
