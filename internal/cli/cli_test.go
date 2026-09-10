package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amansk/spothero-pp-cli/internal/auth"
	"github.com/amansk/spothero-pp-cli/internal/cli"
	"github.com/amansk/spothero-pp-cli/internal/client"
)

func mockHTTP(t *testing.T) *client.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search-params/":
			_, _ = w.Write([]byte(`{"data":{"latitude":41.88,"longitude":-87.62,"starts":"2026-09-11T09:30","ends":"2026-09-11T12:30","sort":"distance","sort_order":"asc","distance_lt":1609}}`))
		case "/facilities/":
			_, _ = w.Write([]byte(`{"data":{"results":[{"id":1,"title":"Lot","price":"$5","distance":50}]}}`))
		case "/user/":
			_, _ = w.Write([]byte(`{"data":{"id":1,"email":"user@example.com"}}`))
		case "/reservations/":
			_, _ = w.Write([]byte(`{"data":{"results":[{"id":"r1","status":"upcoming","facility_title":"Lot","starts":"2026-09-11T09:30","price":"$5"}]}}`))
		case "/reservations/r1/":
			_, _ = w.Write([]byte(`{"data":{"id":"r1","status":"upcoming","cancellable":true}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := client.New(&auth.Session{RawCookieHeader: "s=1"})
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	return c
}

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	root := cli.NewRootForTest(&cli.Options{
		HTTP: mockHTTP(t),
	})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	code := 0
	if err := root.Execute(); err != nil {
		code = cli.ExitCodeForTest(err)
	}
	return code, stdout.String(), stderr.String()
}

func TestVersion(t *testing.T) {
	code, out, _ := runCLI(t, "--version")
	if code != 0 || out == "" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestSearchJSON(t *testing.T) {
	code, out, errOut := runCLI(t, "--json", "search", "--lat", "41.88", "--lng", "-87.62", "--starts", "2026-09-11T09:30", "--ends", "2026-09-11T12:30")
	if code != 0 {
		t.Fatalf("code=%d err=%q out=%q", code, errOut, out)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatal(err)
	}
}

func TestBookPlaceRefusesWithoutGates(t *testing.T) {
	code, _, errOut := runCLI(t, "book", "place", "--facility-id", "1", "--starts", "2026-09-11T09:30", "--ends", "2026-09-11T12:30")
	if code != 2 {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestBookPlaceRequiresExactConfirm(t *testing.T) {
	code, _, _ := runCLI(t, "book", "place", "--facility-id", "1", "--starts", "2026-09-11T09:30", "--ends", "2026-09-11T12:30",
		"--enable-live-booking", "--owner-approved", "--confirm", "WRONG")
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func TestCancelRequiresToken(t *testing.T) {
	code, _, _ := runCLI(t, "cancel", "r1")
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func TestDoctor(t *testing.T) {
	home := t.TempDir()
	root := cli.NewRootForTest(&cli.Options{HTTP: mockHTTP(t), Home: home})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"doctor", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}
