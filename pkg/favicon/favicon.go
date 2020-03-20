package favicon

// draws heavily on Scott Werner's python package https://github.com/scottwernervt/favicon

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Icon struct {
	width     int
	height    int
	RemoteUrl string
	FilePath  string
	mimeType  string
	size      int64
	PageTitle string
}

var LinkRels = [4]string{"icon", "shortcut icon", "apple-touch-icon", "apple-touch-icon-precomposed"}
var MetaNames = [2]string{"msapplication-TileImage", "og:image"}



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
	c := http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := c.Do(&req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errors.New("received bad status code")
	}

	var icons []*Icon
	defIco, err := defaultIcon(resp.Request.URL)
	if err == nil {
		icons = append(icons, defIco)
	}
	tmIcons, err := tagMetaIcons(*resp)
	if err == nil {
		icons = append(icons, tmIcons...)
	}
	if len(icons) == 0 {
		return nil, errors.New("failed to get any icons for website")
	}
	log.Printf("favicon finder got a total of %d icons to choose from", len(icons))

	// get the largest favicon
	sort.Slice(icons, func(i, j int) bool {
		return icons[i].size > icons[j].size
	})

	bestIcon := icons[0]
	bestIcon.PageTitle = getTitle(*resp)
	return bestIcon, nil
}

func getTitle(response http.Response) string {
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Find("title").First().Text())
}

func tagMetaIcons(response http.Response) ([]*Icon, error) {
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, err
	}
	var tagsToFetch []*goquery.Selection

	// handle link nodes
	doc.Find("link").Each(func(i int, s *goquery.Selection) {
		rel, exists := s.Attr("rel")
		if !exists {
			return
		}
		rel = strings.ToLower(rel)
		for _, lr := range LinkRels {
			if rel == lr {
				tagsToFetch = append(tagsToFetch, s)
				return

			}
		}
	})
	// handle meta nodes
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		metaType, exists := s.Attr("name")
		if !exists {
			metaType, exists = s.Attr("property")
			if !exists {
				return
			}
		}
		metaType = strings.ToLower(metaType)
		for _, mn := range MetaNames {
			if metaType == strings.ToLower(mn) {
				tagsToFetch = append(tagsToFetch, s)
			}
		}
	})

	log.Printf("Got %d icon tags for RemoteUrl %s", len(tagsToFetch), response.Request.URL.String())

	var icons []*Icon
	for _, s := range tagsToFetch {
		linkUrl, err := tagMetaUrlGetter(s)
		if err != nil {
			continue
		}
		trimmedLinkUrl := strings.TrimSpace(*linkUrl)
		if trimmedLinkUrl == "" || strings.HasPrefix(trimmedLinkUrl, "data:image/") {
			continue
		}

		parsedLinkUrl, err := url.Parse(trimmedLinkUrl)
		if err != nil {
			continue
		}

		if !parsedLinkUrl.IsAbs() {
			parsedLinkUrl.Host = response.Request.URL.Host
			parsedLinkUrl.Scheme = response.Request.URL.Scheme
		}

		width, height := dimensions(s, linkUrl)

		downloadedIcon, err := getIconFromUrl(parsedLinkUrl)
		if err != nil {
			continue
		}
		icons = append(icons, &Icon{
			width:     width,
			height:    height,
			RemoteUrl: downloadedIcon.RemoteUrl,
			FilePath:  downloadedIcon.FilePath,
			mimeType:  downloadedIcon.mimeType,
			size:      downloadedIcon.size,
		})
	}
	return icons, nil
}

func dimensions(t *goquery.Selection, linkUrl *string) (int, int) {
	// Get icon dimensions from size attribute or filename
	sizes, exists := t.Attr("sizes")
	var width, height string
	if exists && sizes != "any" {
		size := strings.Split(sizes, " ")
		sort.Sort(sort.Reverse(sort.StringSlice(size)))
		re := regexp.MustCompile("[x\xd7]")
		unpack(re.Split(size[0], 2), &width, &height)
	} else {
		sizeRE := regexp.MustCompile("(?mi)(?P<width>[[:digit:]]{2,4})x(?P<height>[[:digit:]]{2,4})")
		matches := sizeRE.FindStringSubmatch(*linkUrl)
		if matches != nil {
			unpack(matches, &width, &height)
		} else {
			width, height = "0", "0"
		}

	}
	// repair bad attribute values e.g. 192x192+
	numRE := regexp.MustCompile("[0-9]+")
	width = numRE.FindString(width)
	height = numRE.FindString(height)

	widthInt, err := strconv.Atoi(width)
	if err != nil {
		widthInt = 0
	}
	heightInt, err := strconv.Atoi(height)
	if err != nil {
		heightInt = 0
	}
	// todo: support og:image:width content=1000 - requires doc level lookups

	return widthInt, heightInt
}

func tagMetaUrlGetter(tm *goquery.Selection) (*string, error) {
	href, exists := tm.Attr("href")
	if exists {
		return &href, nil
	}
	content, exists := tm.Attr("content")
	if exists {
		return &content, nil
	}
	return nil, errors.New("no RemoteUrl found on html tag")
}

func responseToFile(response http.Response, extension string) (string, *int64) {
	file, err := ioutil.TempFile("", fmt.Sprintf("portfall*%s", extension))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	size, err := io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}
	return file.Name(), &size
}

func defaultIcon(parsedUrl *url.URL) (*Icon, error) {
	faviconUrl := url.URL{
		Scheme: parsedUrl.Scheme,
		Host:   parsedUrl.Host,
		Path:   "favicon.ico",
	}
	icon, err := getIconFromUrl(&faviconUrl)
	return icon, err
}

func getIconFromUrl(iconUrl *url.URL) (*Icon, error) {
	// download icon and get extension and size
	log.Printf("Getting icon from RemoteUrl %s", iconUrl.String())
	c := http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := c.Get(iconUrl.String())
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf(
			"received bad status code %d attempting to retrieve icon at %s", resp.StatusCode, iconUrl.String())
	}
	defer resp.Body.Close()

	mimeType := resp.Header.Get("content-type")
	extensions, err := mime.ExtensionsByType(mimeType)
	if err != nil {
		return nil, err
	}
	var extension string
	if extensions != nil {
		extension = extensions[0]
	}
	fp, size := responseToFile(*resp, extension)
	return &Icon{
		width:     0,
		height:    0,
		RemoteUrl: iconUrl.String(),
		FilePath:  fp,
		mimeType:  mimeType,
		size:      *size,
	}, nil
}

func unpack(s []string, vars ...*string) {
	for i, str := range s {
		*vars[i] = str
	}
}
