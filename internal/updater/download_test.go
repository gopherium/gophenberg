// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"crypto/ed25519"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// archivePath is where every test site serves the archive.
const archivePath = "/gophenberg_0.21.0_linux_amd64.tar.gz"

// archiveBody is the archive every download test fetches.
var archiveBody = []byte("the archive of Gophenberg 0.21.0, built for linux-amd64")

// requests records the headers of each request a test site got.
type requests struct {
	mu      sync.Mutex
	headers []http.Header
}

// record keeps the headers of r.
func (q *requests) record(r *http.Request) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.headers = append(q.headers, r.Header.Clone())
}

// count returns how many requests the site got.
func (q *requests) count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.headers)
}

// startSite starts a test site answering with handler, over https unless plain, closed when t ends.
func startSite(t *testing.T, plain bool, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	if plain {
		server.Start()
	} else {
		server.StartTLS()
	}
	t.Cleanup(server.Close)
	return server
}

// serving returns a handler that records each request in seen and answers body.
func serving(body []byte, seen *requests) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seen.record(r)
		_, _ = w.Write(body)
	}
}

// redirecting returns a handler that sends every request on to target.
func redirecting(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}
}

// trusting returns the download client giving up after timeout, trusting the certificate the test sites serve.
func trusting(server *httptest.Server, timeout time.Duration) *http.Client {
	client := updater.NewClient(timeout)
	tested, _ := server.Client().Transport.(*http.Transport)
	client.Transport.(*http.Transport).TLSClientConfig = tested.TLSClientConfig
	return client
}

// keys holds the release key, the standby and a stranger's key of one test.
type keys struct {
	release, standby, stranger ed25519.PrivateKey
}

// newKeys returns fresh keys for one test.
func newKeys(t *testing.T) keys {
	t.Helper()
	return keys{release: newSigner(t), standby: newSigner(t), stranger: newSigner(t)}
}

// signers returns the public keys of the release key and the standby.
func (k keys) signers() []ed25519.PublicKey {
	release, _ := k.release.Public().(ed25519.PublicKey)
	standby, _ := k.standby.Public().(ed25519.PublicKey)
	return []ed25519.PublicKey{release, standby}
}

// download returns the download of the archive at address, signed by key, into a partial file of a new folder.
func (k keys) download(t *testing.T, address string, key ed25519.PrivateKey, client *http.Client) updater.Download {
	t.Helper()
	return updater.Download{
		Artifact: artifactFor(t, archiveBody, address, key), Signers: k.signers(),
		Partial: filepath.Join(t.TempDir(), "0.21.0.partial"), Cap: int64(len(archiveBody)), Client: client,
	}
}

// cutShort answers an archive that stops after five bytes of the length it says.
func cutShort(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Length", strconv.Itoa(len(archiveBody)))
	_, _ = w.Write(archiveBody[:5])
}

// fetched returns the checked file download fetches, closed when t ends, failing t when the fetch fails.
func fetched(t *testing.T, download updater.Download) *os.File {
	t.Helper()
	file, err := download.Fetch(t.Context())
	if err != nil {
		t.Fatalf("Fetch() = %v, want nil", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

// wantDownloaded fails t unless the partial file holds the archive and file reads it from its start.
func wantDownloaded(t *testing.T, download updater.Download, file *os.File) {
	t.Helper()
	held, err := os.ReadFile(download.Partial)
	if err != nil || string(held) != string(archiveBody) {
		t.Errorf("the partial file holds %q, %v, want the archive", held, err)
	}
	wantChecked(t, file)
}

// wantChecked fails t unless file reads the archive from where it stands.
func wantChecked(t *testing.T, file *os.File) {
	t.Helper()
	read, err := io.ReadAll(file)
	if err != nil || string(read) != string(archiveBody) {
		t.Errorf("the checked file reads %q, %v, want the archive from its start", read, err)
	}
}

func TestFetchKeepsAnArchiveWhoseChecksumAndSignatureHold(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	site := startSite(t, false, serving(archiveBody, &seen))
	for _, key := range []ed25519.PrivateKey{k.release, k.standby} {
		download := k.download(t, site.URL+archivePath, key, trusting(site, time.Second))
		wantDownloaded(t, download, fetched(t, download))
	}
}

func TestFetchKeepsTheArchiveItCheckedWhenAnotherDownloadOfTheVersionFails(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	site := startSite(t, false, serving(archiveBody, &seen))
	first := k.download(t, site.URL+archivePath, k.release, trusting(site, time.Second))
	checked := fetched(t, first)
	cut := startSite(t, false, cutShort)
	second := first
	second.Artifact.URL = cut.URL + archivePath
	second.Client = trusting(cut, time.Second)

	_, err := second.Fetch(t.Context())

	wantRefused(t, err, "unexpected EOF")
	wantChecked(t, checked)
}

func TestFetchSendsNothingButItsUserAgent(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	site := startSite(t, false, serving(archiveBody, &seen))

	fetched(t, k.download(t, site.URL+archivePath, k.release, trusting(site, time.Second)))

	if seen.count() != 1 {
		t.Fatalf("the site got %d requests, want one", seen.count())
	}
	header := seen.headers[0]
	names := slices.Sorted(func(yield func(string) bool) {
		for name := range header {
			if !yield(name) {
				return
			}
		}
	})
	if !slices.Equal(names, []string{"User-Agent"}) || header.Get("User-Agent") != "Gophenberg/"+version.Version() {
		t.Errorf("the request carried %v, want only User-Agent: Gophenberg/%s", header, version.Version())
	}
}

func TestFetchFollowsARedirectToHTTPS(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	assets := startSite(t, false, serving(archiveBody, &seen))
	moved := startSite(t, false, redirecting(assets.URL+archivePath))
	download := k.download(t, moved.URL+archivePath, k.release, trusting(moved, time.Second))

	wantDownloaded(t, download, fetched(t, download))
}

func TestFetchRefusesARedirectToPlainHTTP(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	plain := startSite(t, true, serving(archiveBody, &seen))
	moved := startSite(t, false, redirecting(plain.URL+archivePath))
	download := k.download(t, moved.URL+archivePath, k.release, trusting(moved, time.Second))

	_, err := download.Fetch(t.Context())

	wantRefused(t, err, "refused a redirect to "+plain.URL+archivePath+", which is not https")
	if seen.count() != 0 {
		t.Errorf("the plain HTTP site got %d requests, want the redirect refused before it", seen.count())
	}
	wantMissing(t, download.Partial)
}

func TestFetchStopsAfterTenRedirects(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	loop := startSite(t, false, redirecting(archivePath))

	_, err := k.download(t, loop.URL+archivePath, k.release, trusting(loop, time.Second)).Fetch(t.Context())

	wantRefused(t, err, "stopped after 10 redirects")
}

func TestFetchTakesPlainHTTPOnlyOnLoopback(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	plain := startSite(t, true, serving(archiveBody, &seen))
	download := k.download(t, plain.URL+archivePath, k.release, updater.NewClient(time.Second))
	wantDownloaded(t, download, fetched(t, download))

	remote := k.download(t, "http://example.com"+archivePath, k.release, updater.NewClient(time.Second))
	_, err := remote.Fetch(t.Context())
	wantRefused(t, err, "which must be https, or http on a loopback host")
	wantMissing(t, remote.Partial)
}

func TestFetchRefusesAnArchiveLargerThanTheCap(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	site := startSite(t, false, serving(archiveBody, &seen))
	download := k.download(t, site.URL+archivePath, k.release, trusting(site, time.Second))
	download.Cap--

	_, err := download.Fetch(t.Context())

	wantRefused(t, err, "larger than "+strconv.Itoa(len(archiveBody)-1)+" bytes")
	wantMissing(t, download.Partial)
}

func TestFetchRefusesAnArchiveWhoseChecksumDiffers(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	site := startSite(t, false, serving(archiveBody, &seen))
	download := k.download(t, site.URL+archivePath, k.release, trusting(site, time.Second))
	listed := download.Artifact.Checksum
	download.Artifact.Checksum = artifactFor(t, []byte("another archive"), site.URL, k.release).Checksum

	_, err := download.Fetch(t.Context())

	wantRefused(t, err, "refused the archive, its checksum is "+listed+", not "+download.Artifact.Checksum)
	wantMissing(t, download.Partial)
}

func TestFetchRefusesAnArchiveNoListedKeySigned(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	site := startSite(t, false, serving(archiveBody, &seen))
	download := k.download(t, site.URL+archivePath, k.stranger, trusting(site, time.Second))

	_, err := download.Fetch(t.Context())

	wantRefused(t, err, "refused the archive, no release or standby key the key list names signed it")
	wantMissing(t, download.Partial)
}

func TestFetchRefusesAMissingArchive(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	site := startSite(t, false, http.NotFound)
	download := k.download(t, site.URL+archivePath, k.release, trusting(site, time.Second))

	_, err := download.Fetch(t.Context())

	wantRefused(t, err, "the site answered 404 Not Found")
	wantMissing(t, download.Partial)
}

func TestFetchRefusesAnArchiveCutShort(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	site := startSite(t, false, cutShort)
	download := k.download(t, site.URL+archivePath, k.release, trusting(site, time.Second))

	_, err := download.Fetch(t.Context())

	wantRefused(t, err, "unexpected EOF")
	wantMissing(t, download.Partial)
}

func TestFetchRefusesASiteThatAnswersTooLate(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	done := make(chan struct{})
	site := startSite(t, false, func(http.ResponseWriter, *http.Request) {
		<-done
	})
	t.Cleanup(func() { close(done) })
	download := k.download(t, site.URL+archivePath, k.release, trusting(site, 100*time.Millisecond))

	_, err := download.Fetch(t.Context())

	wantRefused(t, err, "Client.Timeout exceeded")
	wantMissing(t, download.Partial)
}

func TestFetchRefusesAPartialFileItCannotWrite(t *testing.T) {
	t.Parallel()

	k := newKeys(t)
	var seen requests
	site := startSite(t, false, serving(archiveBody, &seen))
	download := k.download(t, site.URL+archivePath, k.release, trusting(site, time.Second))
	download.Partial = filepath.Join(t.TempDir(), "downloads", "0.21.0.partial")

	_, err := download.Fetch(t.Context())

	wantRefused(t, err, "write the download")
	wantMissing(t, filepath.Dir(download.Partial))
}
