package favicon

import (
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"net/url"
	"os"
	"path"
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

func TestGetTitle(t *testing.T)  {
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
