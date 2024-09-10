package dl

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

var sub_p *tea.Program

type progressWriter struct {
	total      int
	downloaded int
	file       *os.File
	reader     io.Reader
	onProgress func(float64)
}

func (pw *progressWriter) Start() {
	_, err := io.Copy(pw.file, io.TeeReader(pw.reader, pw))
	if err != nil {
		sub_p.Send(progressErrMsg{err})
	}
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	pw.downloaded += len(p)
	if pw.total > 0 && pw.onProgress != nil {
		pw.onProgress(float64(pw.downloaded) / float64(pw.total))
	}
	return len(p), nil
}

func getResponse(url string) (*http.Response, error) {
	resp, err := http.Get(url) // nolint:gosec
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("receiving status of %d for url: %s", resp.StatusCode, url)
	}
	return resp, nil
}

func DownloadWheel(url string) (string, error) {
	resp, err := getResponse(url)
	if err != nil {
		log.Fatal("Could not get a response")
		os.Exit(1)
	}

	defer resp.Body.Close()
	if resp.ContentLength <= 0 {
		log.Fatal("Cannot parse content length. Aborting download")
		os.Exit(1)
	}

	filename := strings.Split(filepath.Base(url), "#")[0]

	file, err := os.Create(os.Getenv("HOME") + "/Downloads/" + filename)
	if err != nil {
		//return "", err
		log.Fatal("Could not save file on disk")
	}
	defer file.Close()

	// Write the body to file
	pw := &progressWriter{
		total:  int(resp.ContentLength),
		file:   file,
		reader: resp.Body,
		onProgress: func(ratio float64) {
			sub_p.Send(progressMsg(ratio))
		},
	}

	mdl := model{
		pw:       pw,
		progress: progress.New(progress.WithSolidFill("#e28743")),
	}
	// Start Bubble Tea
	sub_p = tea.NewProgram(mdl)

	// Start the download
	go pw.Start()
	if _, err := sub_p.Run(); err != nil {
		log.Println("error running program:", err)
		os.Exit(1)
	}

	return file.Name(), nil
}
