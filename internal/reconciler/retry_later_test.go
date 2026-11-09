package reconciler

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RetryLater", func() {
	It("returns the delay of a wrapped retry later error", func() {
		after, ok := RetryLater(fmt.Errorf("csc: %w", &RetryLaterError{Reason: "busy", RequeueAfter: time.Minute}))
		Expect(ok).To(BeTrue())
		Expect(after).To(Equal(time.Minute))
	})

	It("returns the longest delay of joined retry later errors", func() {
		after, ok := RetryLater(errors.Join(
			&RetryLaterError{Reason: "a", RequeueAfter: time.Minute},
			&RetryLaterError{Reason: "b", RequeueAfter: 3 * time.Minute},
		))
		Expect(ok).To(BeTrue())
		Expect(after).To(Equal(3 * time.Minute))
	})

	It("returns false if any other error is contained", func() {
		_, ok := RetryLater(errors.Join(&RetryLaterError{Reason: "a", RequeueAfter: time.Minute}, errors.New("boom")))
		Expect(ok).To(BeFalse())
	})

	It("returns false for nil and other errors", func() {
		_, ok := RetryLater(nil)
		Expect(ok).To(BeFalse())
		_, ok = RetryLater(errors.New("boom"))
		Expect(ok).To(BeFalse())
	})

	It("includes the underlying error in the message", func() {
		err := &RetryLaterError{Reason: "busy", Err: errors.New("409")}
		Expect(err.Error()).To(Equal("busy: 409"))
		Expect(errors.Unwrap(err)).To(MatchError("409"))
	})
})
