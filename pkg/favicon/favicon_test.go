package favicon

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
)

// todo: create a mock server for these reqs to hit

func TestDefaultIcon(t *testing.T) {
	goog, _ := url.Parse("http://www.google.com")
	icon, err := defaultIcon(goog)
	if err != nil || icon == nil {
		t.Error(err)
		return
	}
	if icon.RemoteUrl != "http://www.google.com/favicon.ico" {
		t.Errorf("icon RemoteUrl wrong, expected http://www.google.com/favicon.ico, got %s", icon.RemoteUrl)
	}
	if path.Dir(icon.FilePath) != os.TempDir() {
		t.Errorf("expected output to live in os tempdir %s got %s", os.TempDir(), path.Dir(icon.FilePath))
	}
	if icon.size == 0 {
		t.Errorf("expected non-zero file iconsize")
	}
}

func TestTagMetaIcons(t *testing.T) {
	goog, _ := url.Parse("http://www.google.com")
	resp, err := http.DefaultClient.Get(goog.String())
	if err != nil {
		t.Errorf("Connectivity problem")
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		t.Error(err)
	}
	tmIcons, err := tagMetaIcons(*doc, *resp.Request.URL)
	if err != nil {
		t.Error(err)
	}
	if len(tmIcons) != 1 {
		t.Errorf("Got %d icons, expected 1", len(tmIcons))
	}else{
		icon := tmIcons[0]
		if icon.size == 0 {
			t.Errorf("expected non-zero file iconsize")
		}
		if path.Dir(icon.FilePath) != os.TempDir() {
			t.Errorf("expected output to live in os tempdir %s got %s", os.TempDir(), path.Dir(icon.FilePath))
		}
		if icon.width != 0 || icon.height != 0 {
			t.Errorf("expected 0,0 height,width. Got %d,%d", icon.width, icon.height)
		}
	}


}


func TestTagMetaIconsWithNoFavicon(t *testing.T) {
	raw := `
<!DOCTYPE html>
<html lang="en">
  <head>
    <script>
      
      !(function() {
        if ('PerformanceLongTaskTiming' in window) {
          var g = (window.__tti = { e: [] });
          g.o = new PerformanceObserver(function(l) {
            g.e = g.e.concat(l.getEntries());
          });
          g.o.observe({ entryTypes: ['longtask'] });
        }
      })();

    </script>
    <meta charset="utf-8" />
    <meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1" />
    <meta name="viewport" content="width=device-width" />
    <meta name="theme-color" content="#000" />

    <title>Grafana</title>

    <base href="/" />

    <link
      rel="preload"
      href="public/fonts/roboto/RxZJdnzeo3R5zSexge8UUVtXRa8TVwTICgirnJhmVJw.woff2"
      as="font"
      crossorigin
    />

    <link rel="icon" type="image/png" href="public/img/fav32.png" />
    <link rel="apple-touch-icon" sizes="180x180" href="public/img/apple-touch-icon.png" />
    <link rel="mask-icon" href="public/img/grafana_mask_icon.svg" color="#F05A28" />

    <link rel="stylesheet" href="public/build/grafana.dark.71af4420a0fca6a7ffc6.css" />

    <script>
      performance.mark('css done blocking');
    </script>
    <meta name="apple-mobile-web-app-capable" content="yes" />
    <meta name="apple-mobile-web-app-status-bar-style" content="black" />
    <meta name="msapplication-TileColor" content="#2b5797" />
    <meta name="msapplication-config" content="public/img/browserconfig.xml" />
  </head>

  <body class="theme-dark app-grafana">
  </body>
</html>
`
	reader := strings.NewReader(raw)
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		t.Error(err)
	}
	tmIcons, err := tagMetaIcons(*doc, url.URL{
		Scheme: "https",
		Host: "play.grafana.org",
	})
	if err != nil {
		t.Error(err)
	}
	if len(tmIcons) != 2 {
		t.Errorf("Got %d icons, expected 1", len(tmIcons))
	}

	icon := tmIcons[0]
	if icon.size == 0 {
		t.Errorf("expected non-zero file iconsize")
	}
	if path.Dir(icon.FilePath) != os.TempDir() {
		t.Errorf("expected output to live in os tempdir %s got %s", os.TempDir(), path.Dir(icon.FilePath))
	}
	if icon.width != 0 || icon.height != 0 {
		t.Errorf("expected 0,0 height,width. Got %d,%d", icon.width, icon.height)
	}
}

func TestGetTitle(t *testing.T) {
	goog, _ := url.Parse("http://www.google.com")
	resp, err := http.DefaultClient.Get(goog.String())
	if err != nil {
		t.Error(err)
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		t.Error(err)
	}
	title := getTitle(*doc, *resp.Request.URL)
	if title != "Google" {
		t.Errorf("Expected page title Google but got %s", title)
	}
}

func TestGetBest(t *testing.T) {
	goog, _ := url.Parse("http://www.google.com")
	icon, err := GetBest(goog.String())
	if err != nil {
		t.Error(err)
	}
	if icon.size == 0 {
		t.Errorf("expected non-zero file iconsize")
	}
	if path.Dir(icon.FilePath) != os.TempDir() {
		t.Errorf("expected output to live in os tempdir %s got %s", os.TempDir(), path.Dir(icon.FilePath))
	}
	if icon.width != 0 || icon.height != 0 {
		t.Errorf("expected 0,0 height,width. Got %d,%d", icon.width, icon.height)
	}

	defaultIcon, err := defaultIcon(goog)
	if err != nil {
		t.Error(err)
	}
	// best icon is actually chosen
	if defaultIcon.RemoteUrl == icon.RemoteUrl || defaultIcon.size == icon.size {
		t.Errorf("Best icon not chosen. Favicon chosen.")
	}
	if icon.PageTitle != "Google" {
		t.Errorf("Expected page title Google but got %s", icon.PageTitle)
	}
}
