package httpbin

import (
	"fmt"
	"net/http"
	"os"
	"time"

	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"

	"github.com/DataDog/datadog-go/statsd"
)

// Default configuration values
const (
	DefaultMaxBodySize int64 = 1024 * 1024
	DefaultMaxDuration       = 10 * time.Second
	DefaultHostname          = "go-httpbin"
)

// DefaultParams defines default parameter values
type DefaultParams struct {
	DripDuration time.Duration
	DripDelay    time.Duration
	DripNumBytes int64
}

// DefaultDefaultParams defines the DefaultParams that are used by default. In
// general, these should match the original httpbin.org's defaults.
var DefaultDefaultParams = DefaultParams{
	DripDuration: 2 * time.Second,
	DripDelay:    2 * time.Second,
	DripNumBytes: 10,
}

type headersProcessorFunc func(h http.Header) http.Header

// HTTPBin contains the business logic
type HTTPBin struct {
	// Max size of an incoming request generated response body, in bytes
	MaxBodySize int64

	// Max duration of a request, for those requests that allow user control
	// over timing (e.g. /delay)
	MaxDuration time.Duration

	// Observer called with the result of each handled request
	Observer Observer

	// Default parameter values
	DefaultParams DefaultParams

	// Set of hosts to which the /redirect-to endpoint will allow redirects
	AllowedRedirectDomains map[string]struct{}

	forbiddenRedirectError string

	// The hostname to expose via /hostname.
	hostname string

	// The app's http handler
	handler http.Handler

	excludeHeadersProcessor headersProcessorFunc
}

// New creates a new HTTPBin instance
func New(opts ...OptionFunc) *HTTPBin {
	h := &HTTPBin{
		MaxBodySize:   DefaultMaxBodySize,
		MaxDuration:   DefaultMaxDuration,
		DefaultParams: DefaultDefaultParams,
		hostname:      DefaultHostname,
	}
	for _, opt := range opts {
		opt(h)
	}
	h.handler = h.Handler()
	return h
}

// ServeHTTP implememnts the http.Handler interface.
func (h *HTTPBin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

// Assert that HTTPBin implements http.Handler interface
var _ http.Handler = &HTTPBin{}

func getLatencyHandler(statsd *statsd.Client) func(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
	if statsd == nil {
		// If no statsd exists, return the handler
		return func(_ string, handler http.HandlerFunc) http.HandlerFunc {
			return handler
		}
	}
	envTag := "environment:" + os.Getenv("DD_ENV")
	metricName := os.Getenv("DD_SERVICE") + ".timer"
	return func(endpoint string, handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			// Call the original handler
			handler(w, r)

			endTime := time.Now()
			latency := endTime.Sub(startTime).Nanoseconds()

			resourceNameTag := fmt.Sprintf("resource_name:%s_%s", r.Method, endpoint)
			statsd.Histogram(metricName, float64(latency)/1000000, []string{envTag, resourceNameTag}, 1)
		}
	}
}

// Handler returns an http.Handler that exposes all HTTPBin endpoints
func (h *HTTPBin) Handler() http.Handler {
	mux := httptrace.NewServeMux()

	statsd, err := statsd.New(os.Getenv("STATSD_ADDR") + ":8125")
	if err != nil {
		fmt.Printf("no statsd: %s\n", err)
	}

	wrapper := getLatencyHandler(statsd)

	mux.HandleFunc("/", wrapper("/", methods(h.Index, "GET")))
	mux.HandleFunc("/forms/post", wrapper("/forms/post", methods(h.FormsPost, "GET")))
	mux.HandleFunc("/encoding/utf8", wrapper("/encoding/utf8", methods(h.UTF8, "GET")))

	mux.HandleFunc("/delete", wrapper("/delete", methods(h.RequestWithBody, "DELETE")))
	mux.HandleFunc("/get", wrapper("/get", methods(h.Get, "GET")))
	mux.HandleFunc("/head", wrapper("/head", methods(h.Get, "HEAD")))
	mux.HandleFunc("/patch", wrapper("/patch", methods(h.RequestWithBody, "PATCH")))
	mux.HandleFunc("/post", wrapper("/post", methods(h.RequestWithBody, "POST")))
	mux.HandleFunc("/put", wrapper("/put", methods(h.RequestWithBody, "PUT")))

	mux.HandleFunc("/anything", wrapper("/anything", h.Anything))
	mux.HandleFunc("/anything/", wrapper("/anything/", h.Anything))

	mux.HandleFunc("/ip", wrapper("/ip", h.IP))
	mux.HandleFunc("/user-agent", wrapper("/user-agent", h.UserAgent))
	mux.HandleFunc("/headers", wrapper("/headers", h.Headers))
	mux.HandleFunc("/response-headers", wrapper("/response-headers", h.ResponseHeaders))
	mux.HandleFunc("/hostname", wrapper("/hostname", h.Hostname))

	mux.HandleFunc("/status/", wrapper("/status/", h.Status))
	mux.HandleFunc("/unstable", wrapper("/unstable", h.Unstable))

	mux.HandleFunc("/redirect/", wrapper("/redirect/", h.Redirect))
	mux.HandleFunc("/relative-redirect/", wrapper("/relative-redirect/", h.RelativeRedirect))
	mux.HandleFunc("/absolute-redirect/", wrapper("/absolute-redirect/", h.AbsoluteRedirect))
	mux.HandleFunc("/redirect-to", wrapper("/redirect-to", h.RedirectTo))

	mux.HandleFunc("/cookies", wrapper("/cookies", h.Cookies))
	mux.HandleFunc("/cookies/set", wrapper("/cookies/set", h.SetCookies))
	mux.HandleFunc("/cookies/delete", wrapper("/cookies/delete", h.DeleteCookies))

	mux.HandleFunc("/basic-auth/", wrapper("/basic-auth/", h.BasicAuth))
	mux.HandleFunc("/hidden-basic-auth/", wrapper("/hidden-basic-auth/", h.HiddenBasicAuth))
	mux.HandleFunc("/digest-auth/", wrapper("/digest-auth/", h.DigestAuth))
	mux.HandleFunc("/bearer", wrapper("/bearer", h.Bearer))

	mux.HandleFunc("/deflate", wrapper("/deflate", h.Deflate))
	mux.HandleFunc("/gzip", wrapper("/gzip", h.Gzip))

	mux.HandleFunc("/stream/", wrapper("/stream/", h.Stream))
	mux.HandleFunc("/delay/", wrapper("/delay/", h.Delay))
	mux.HandleFunc("/drip", wrapper("/drip", h.Drip))

	mux.HandleFunc("/range/", wrapper("/range/", h.Range))
	mux.HandleFunc("/bytes/", wrapper("/bytes/", h.Bytes))
	mux.HandleFunc("/stream-bytes/", wrapper("/stream-bytes/", h.StreamBytes))

	mux.HandleFunc("/html", wrapper("/html", h.HTML))
	mux.HandleFunc("/robots.txt", wrapper("/robots.txt", h.Robots))
	mux.HandleFunc("/deny", wrapper("/deny", h.Deny))

	mux.HandleFunc("/cache", wrapper("/cache", h.Cache))
	mux.HandleFunc("/cache/", wrapper("/cache/", h.CacheControl))
	mux.HandleFunc("/etag/", wrapper("/etag/", h.ETag))

	mux.HandleFunc("/links/", wrapper("/links/", h.Links))

	mux.HandleFunc("/image", wrapper("/image", h.ImageAccept))
	mux.HandleFunc("/image/", wrapper("/image/", h.Image))
	mux.HandleFunc("/xml", wrapper("/xml", h.XML))
	mux.HandleFunc("/json", wrapper("/json", h.JSON))

	mux.HandleFunc("/uuid", wrapper("/uuid", h.UUID))
	mux.HandleFunc("/base64/", wrapper("/base64/", h.Base64))

	mux.HandleFunc("/dump/request", wrapper("/dump/request", h.DumpRequest))

	// existing httpbin endpoints that we do not support
	mux.HandleFunc("/brotli", notImplementedHandler)

	// Make sure our ServeMux doesn't "helpfully" redirect these invalid
	// endpoints by adding a trailing slash. See the ServeMux docs for more
	// info: https://golang.org/pkg/net/http/#ServeMux
	mux.HandleFunc("/absolute-redirect", http.NotFound)
	mux.HandleFunc("/basic-auth", http.NotFound)
	mux.HandleFunc("/delay", http.NotFound)
	mux.HandleFunc("/digest-auth", http.NotFound)
	mux.HandleFunc("/hidden-basic-auth", http.NotFound)
	mux.HandleFunc("/redirect", http.NotFound)
	mux.HandleFunc("/relative-redirect", http.NotFound)
	mux.HandleFunc("/status", http.NotFound)
	mux.HandleFunc("/stream", http.NotFound)
	mux.HandleFunc("/bytes", http.NotFound)
	mux.HandleFunc("/stream-bytes", http.NotFound)
	mux.HandleFunc("/links", http.NotFound)

	// Apply global middleware
	var handler http.Handler
	handler = mux
	handler = limitRequestSize(h.MaxBodySize, handler)
	handler = preflight(handler)
	handler = autohead(handler)
	if h.Observer != nil {
		handler = observe(h.Observer, handler)
	}

	return handler
}

func (h *HTTPBin) setExcludeHeaders(excludeHeaders string) {
	regex := createFullExcludeRegex(excludeHeaders)
	if regex != nil {
		h.excludeHeadersProcessor = createExcludeHeadersProcessor(regex)
	}
}
