package ghclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeGitHub simulates api.github.com on transport level.
type fakeGitHub struct {
	tokenCalls atomic.Int32
	apiCalls   atomic.Int32
	// tokenErrs are returned (in order) for the first access_tokens calls.
	tokenErrs []error
	// apiErrs are returned (in order) for the first API calls.
	apiErrs []error
	// unauthorizedFor rejects API calls that carry this Authorization header value.
	unauthorizedFor string
	// tokens returned by successive access_tokens calls (defaults to "tok").
	tokens []string
	// seenAuth records the Authorization header of every API call.
	seenAuth []string
}

func (f *fakeGitHub) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, "/access_tokens") {
		n := int(f.tokenCalls.Add(1))
		if n <= len(f.tokenErrs) {
			return nil, f.tokenErrs[n-1]
		}
		tok := "tok"
		if n-1 < len(f.tokens) {
			tok = f.tokens[n-1]
		}
		body := fmt.Sprintf(`{"token":%q,"expires_at":%q}`, tok, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: req}, nil
	}
	n := int(f.apiCalls.Add(1))
	f.seenAuth = append(f.seenAuth, req.Header.Get("Authorization"))
	if n <= len(f.apiErrs) {
		return nil, f.apiErrs[n-1]
	}
	if f.unauthorizedFor != "" && req.Header.Get("Authorization") == f.unauthorizedFor {
		return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{}, Request: req}, nil
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{}, Request: req}, nil
}

var _ = Describe("AuthorizeGitHubAccess", func() {
	var key *rsa.PrivateKey

	BeforeEach(func() {
		var err error
		key, err = rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).NotTo(HaveOccurred())
	})

	get := func(rt http.RoundTripper, path string) error {
		req, err := http.NewRequest(http.MethodGet, "https://api.github.com"+path, nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err := rt.RoundTrip(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		return err
	}

	It("requests the installation token only once for multiple requests", func() {
		gh := &fakeGitHub{}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		for range 10 {
			Expect(get(rt, "/orgs/o/members")).To(Succeed())
		}

		Expect(gh.tokenCalls.Load()).To(Equal(int32(1)))
		Expect(gh.apiCalls.Load()).To(Equal(int32(10)))
	})

	It("retries the token request on HTTP/2 PROTOCOL_ERROR", func() {
		gh := &fakeGitHub{tokenErrs: []error{errors.New("stream error: stream ID 219; PROTOCOL_ERROR; received from peer")}}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		Expect(get(rt, "/orgs/o/members")).To(Succeed())
		Expect(gh.tokenCalls.Load()).To(Equal(int32(2)))
	})

	It("retries idempotent API requests on connection reset", func() {
		gh := &fakeGitHub{apiErrs: []error{io.ErrUnexpectedEOF}}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		Expect(get(rt, "/orgs/o/members")).To(Succeed())
		Expect(gh.apiCalls.Load()).To(Equal(int32(2)))
	})

	It("gives up after the maximum number of retries", func() {
		errs := []error{io.EOF, io.EOF, io.EOF, io.EOF, io.EOF}
		gh := &fakeGitHub{apiErrs: errs}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		Expect(get(rt, "/orgs/o/members")).To(HaveOccurred())
		Expect(gh.apiCalls.Load()).To(Equal(int32(transportRetryMaxRetries + 1)))
	})

	It("does not retry non-idempotent requests", func() {
		gh := &fakeGitHub{apiErrs: []error{io.EOF}}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		req, err := http.NewRequest(http.MethodPost, "https://api.github.com/orgs/o/repos", strings.NewReader("{}"))
		Expect(err).NotTo(HaveOccurred())
		_, err = rt.RoundTrip(req)

		Expect(err).To(HaveOccurred())
		Expect(gh.apiCalls.Load()).To(Equal(int32(1)))
	})

	It("does not retry context cancellation", func() {
		gh := &fakeGitHub{apiErrs: []error{fmt.Errorf("wrapped: %w", context.Canceled)}}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		Expect(get(rt, "/orgs/o/members")).To(HaveOccurred())
		Expect(gh.apiCalls.Load()).To(Equal(int32(1)))
	})

	It("discards a rejected token on 401 and replays the request with a new token", func() {
		gh := &fakeGitHub{tokens: []string{"old", "new"}, unauthorizedFor: "token old"}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		Expect(get(rt, "/orgs/o/members")).To(Succeed())

		Expect(gh.tokenCalls.Load()).To(Equal(int32(2)))
		Expect(gh.seenAuth).To(Equal([]string{"token old", "token new"}))

		// the new token is cached from now on
		Expect(get(rt, "/orgs/o/members")).To(Succeed())
		Expect(gh.tokenCalls.Load()).To(Equal(int32(2)))
	})

	It("replays a request with a body on 401", func() {
		gh := &fakeGitHub{tokens: []string{"old", "new"}, unauthorizedFor: "token old"}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		req, err := http.NewRequest(http.MethodPatch, "https://api.github.com/orgs/o", strings.NewReader("{}"))
		Expect(err).NotTo(HaveOccurred())
		resp, err := rt.RoundTrip(req)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(gh.seenAuth).To(Equal([]string{"token old", "token new"}))
	})

	It("replays only once if the new token is rejected as well", func() {
		gh := &fakeGitHub{unauthorizedFor: "token tok"}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		req, err := http.NewRequest(http.MethodGet, "https://api.github.com/orgs/o", nil)
		Expect(err).NotTo(HaveOccurred())
		resp, err := rt.RoundTrip(req)

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		Expect(gh.apiCalls.Load()).To(Equal(int32(2)))
	})

	It("does not replay a 401 when the body cannot be replayed, but still drops the token", func() {
		gh := &fakeGitHub{tokens: []string{"old", "new"}, unauthorizedFor: "token old"}
		rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{Resilient: true})

		req, err := http.NewRequest(http.MethodPatch, "https://api.github.com/orgs/o", io.NopCloser(strings.NewReader("{}")))
		Expect(err).NotTo(HaveOccurred())
		resp, err := rt.RoundTrip(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		Expect(gh.apiCalls.Load()).To(Equal(int32(1)))

		Expect(get(rt, "/orgs/o/members")).To(Succeed())
		Expect(gh.seenAuth).To(Equal([]string{"token old", "token new"}))
	})

	Context("default (legacy) mode", func() {
		It("fetches a token for every request and does not retry transport errors", func() {
			gh := &fakeGitHub{apiErrs: []error{io.EOF}}
			rt := AuthorizeGitHubAccess(gh, 1, 2, key, AuthOptions{})

			Expect(get(rt, "/orgs/o/members")).To(HaveOccurred())
			Expect(get(rt, "/orgs/o/members")).To(Succeed())

			Expect(gh.tokenCalls.Load()).To(Equal(int32(2)))
			Expect(gh.apiCalls.Load()).To(Equal(int32(2)))
		})
	})
})
