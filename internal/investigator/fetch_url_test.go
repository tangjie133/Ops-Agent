package investigator

import (
	"testing"
)

func TestValidateFetchURL(t *testing.T) {
	if _, err := validateFetchURL("https://wiki.dfrobot.com/test"); err != nil {
		t.Fatal(err)
	}
	if _, err := validateFetchURL("http://127.0.0.1/x"); err == nil {
		t.Fatal("should block localhost")
	}
	if _, err := validateFetchURL("file:///etc/passwd"); err == nil {
		t.Fatal("should block file scheme")
	}
}

func TestExtractHTTPURLs(t *testing.T) {
	text := `See https://wiki.dfrobot.com/dfr0998 and https://github.com/foo/bar/issues/1`
	urls := ExtractHTTPURLs(text)
	if len(urls) < 2 {
		t.Fatalf("urls=%v", urls)
	}
}

func TestParseDDGHTML(t *testing.T) {
	html := `<a rel="nofollow" class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com">Example</a>
<a class="result__snippet">snippet text</a>`
	hits := parseDDGHTML(html, 5)
	if len(hits) != 1 || hits[0].Title != "Example" {
		t.Fatalf("hits=%+v", hits)
	}
	if hits[0].URL != "https://example.com" {
		t.Fatalf("url=%q", hits[0].URL)
	}
}

func TestHTMLToText(t *testing.T) {
	got := htmlToText("<html><body><p>Hello <b>world</b></p></body></html>")
	if got != "Hello world" {
		t.Fatalf("got %q", got)
	}
}
