package ghclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/shurcooL/githubv4"
)

var _ = Describe("GraphQLClientWrapper", func() {
	var (
		server *httptest.Server
		client *GraphQLClientWrapper
	)

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	// newWrapperAgainst spins up a test server returning the given raw GraphQL JSON body and
	// wires a GraphQLClientWrapper (via githubv4.NewEnterpriseClient) to talk to it.
	newWrapperAgainst := func(handler http.HandlerFunc) *GraphQLClientWrapper {
		server = httptest.NewServer(handler)
		v4 := githubv4.NewEnterpriseClient(server.URL, server.Client())
		return NewGraphQLClientWrapper(v4)
	}

	Describe("Query", func() {
		It("executes a query and decodes the response into the provided struct", func() {
			client = newWrapperAgainst(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"octocat"}}}`))
			})

			var q struct {
				Viewer struct {
					Login githubv4.String
				}
			}
			err := client.Query(context.Background(), &q, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(q.Viewer.Login)).To(Equal("octocat"))
		})

		It("surfaces GraphQL errors returned by the API", func() {
			client = newWrapperAgainst(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"errors":[{"message":"boom"}]}`))
			})

			var q struct {
				Viewer struct {
					Login githubv4.String
				}
			}
			err := client.Query(context.Background(), &q, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("boom"))
		})

		It("propagates transport-level HTTP errors", func() {
			client = newWrapperAgainst(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			})

			var q struct {
				Viewer struct {
					Login githubv4.String
				}
			}
			err := client.Query(context.Background(), &q, nil)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Mutate", func() {
		It("executes a mutation and forwards the input", func() {
			var body map[string]any
			client = newWrapperAgainst(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"data":{"addComment":{"clientMutationId":"1"}}}`))
			})

			var m struct {
				AddComment struct {
					ClientMutationID githubv4.String
				} `graphql:"addComment(input: $input)"`
			}
			input := githubv4.AddCommentInput{
				SubjectID: "subject-id",
				Body:      "hello",
			}
			err := client.Mutate(context.Background(), &m, input, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(body).To(HaveKey("query"))
			Expect(body).To(HaveKey("variables"))
		})
	})
})
