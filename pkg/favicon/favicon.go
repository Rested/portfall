package favicon

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
)

type Icon struct {
	width     int
	height    int
	url       string
	filepath  string
	extension string
	size      int64
}

func GetBest(getUrl string) (*Icon, error) {
	parsedUrl, err := url.Parse(getUrl)
	if err != nil {
		return nil, err
	}
	req := http.Request{
		Method: "GET",
		URL:    parsedUrl,
		Proto:  "HTTP",
	}
	resp, err := http.DefaultClient.Do(&req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseUrl, err := resp.Location()
	if err != nil {
		return nil, err
	}
	var icons []*Icon
	defIco, err := defaultIcon(responseUrl)
	if err != nil {
		icons = append(icons, defIco)
	}
	sort.Slice(icons, func(i, j int) bool {
		return icons[i].height * icons[i].width < icons[j].height * icons[j].width
	})
	return icons[0], nil
}

func responseToFile(response http.Response) (string, *int64) {
	file, err := ioutil.TempFile("", "portfall")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	size, err := io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(os.TempDir(), file.Name()), &size
}

func defaultIcon(parsedUrl *url.URL) (*Icon, error) {
	faviconUrl := url.URL{
		Scheme: parsedUrl.Scheme,
		Host:   parsedUrl.Host,
		Path:   "favicon.ico",
	}
	resp, err := http.DefaultClient.Get(faviconUrl.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	fp, size := responseToFile(*resp)

	return &Icon{
		width:     0,
		height:    0,
		url:       faviconUrl.String(),
		filepath:  fp,
		extension: "ico",
		size:      *size,
	}, nil
}
