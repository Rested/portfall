package favicon

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"testing"
)

func TestDefaultIcon(t *testing.T) {
	goog, err := url.Parse("http://www.google.com")
	icon, err := defaultIcon(goog)
	if err != nil || icon == nil {
		t.Error(err)
		return
	}
	if icon.url != "http://www.google.com/favicon.ico" {
		t.Errorf("icon url wrong, expected http://www.google.com/favicon.ico, got %s", icon.url)
	}
	if path.Dir(icon.filepath) != os.TempDir() {
		t.Errorf("expected output to live in os tempdir %s got %s", os.TempDir(), path.Dir(icon.filepath))
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
	tmIcons, err := tagMetaIcons(*resp)
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
	if path.Dir(icon.filepath) != os.TempDir() {
		t.Errorf("expected output to live in os tempdir %s got %s", os.TempDir(), path.Dir(icon.filepath))
	}
	if icon.width != 0 || icon.height != 0 {
		t.Errorf("expected 0,0 height,width. Got %d,%d", icon.width, icon.height)
	}
}


