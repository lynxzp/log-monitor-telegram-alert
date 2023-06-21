package main

import (
	"bufio"
	"github.com/pelletier/go-toml/v2"
	"github.com/radovskyb/watcher"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type watchedFiles struct {
	files map[string]*bufio.Reader
}

type config struct {
	Log struct {
		AlertKeywords []string
	}
	Telegram struct {
		Token           string
		ChatId          int64
		MessageTemplate string
	}
}

var cfg config

type templateData struct {
	Line    string
	File    string
	Keyword string
}

func main() {
	//watchOneFile()
	loadConfig(&cfg, "config.toml")
	telegramInit()
	watching()
}

func watchOneFile() {
	file, err := os.Open("test.log")
	if err != nil {
		log.Fatalln(err)
	}
	r := bufio.NewReader(file)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			//log.Fatalln(err)
		} else {
			log.Printf("Line: %s", line)
		}
	}
}

func loadConfig(cfg interface{}, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	d := toml.NewDecoder(file)
	d.DisallowUnknownFields()
	err = d.Decode(cfg)
	if err != nil {
		panic(err)
	}
}

func watching() {
	watch := watcher.New()
	r := regexp.MustCompile("^.*\\.log$")
	watch.AddFilterHook(watcher.RegexFilterHook(r, false))
	files := newWatchedFiles()

	go catchEvent(watch, files)

	if err := watch.AddRecursive("."); err != nil {
		log.Fatalln(err)
	}

	count := 0
	for path, _ := range watch.WatchedFiles() {
		files.add(path, true)
		count++
	}
	log.Printf("Watching %d files\n", count)

	// Start the watching process - it'll check for changes every 100ms.
	if err := watch.Start(time.Millisecond * 100); err != nil {
		log.Fatalln(err)
	}
}

func catchEvent(w *watcher.Watcher, files *watchedFiles) {
	for {
		select {
		case event := <-w.Event:
			switch event.Op {
			case watcher.Create:
				files.add(event.Path, false)
			case watcher.Write:
				files.check(event.Path)
			default:
				log.Printf("Warn Unknown event: %+v\n", event)
			}
		case err := <-w.Error:
			log.Fatalln(err)
		case <-w.Closed:
			return
		}
	}
}

func (w *watchedFiles) add(filename string, ignore bool) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalln(err)
	}
	r := bufio.NewReader(file)
	if ignore {
		for {
			_, err := r.ReadString('\n')
			if err != nil {
				break
			}
		}
	}
	w.files[filename] = r
}

func newWatchedFiles() *watchedFiles {
	return &watchedFiles{files: make(map[string]*bufio.Reader)}
}

func (w *watchedFiles) check(filename string) {
	r := w.files[filename]
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		for _, keyword := range cfg.Log.AlertKeywords {
			if strings.Contains(line, keyword) {
				data := templateData{Line: line, File: filename, Keyword: keyword}
				// parse line
				t, err := template.New("").Parse(cfg.Telegram.MessageTemplate)
				if err != nil {
					panic(err)
				}
				var buf strings.Builder
				err = t.Execute(&buf, data)
				if err != nil {
					panic(err)
				}
				log.Printf(buf.String())
				sendTelegram(buf.String())
			}
		}
	}
}

var bot *tgbotapi.BotAPI

func telegramInit() {
	var err error
	bot, err = tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		log.Panic(err)
	}
}

func sendTelegram(message string) {
	msg := tgbotapi.NewMessage(cfg.Telegram.ChatId, message)
	rep, err := bot.Send(msg)
	if err != nil {
		log.Println(err, rep)
	}
}
