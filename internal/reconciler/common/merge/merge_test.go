package merge

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InlineSource", func() {
	It("formats the index", func() {
		Expect(InlineSource(3)).To(Equal("inline[3]"))
	})
})

var _ = Describe("Dedupe", func() {
	type item struct {
		key, value, source string
	}

	var duplicates [][2]item

	dedupe := func(items ...item) ([]item, []Conflict) {
		return Dedupe(items,
			func(i item) string { return i.key },
			func(a, b item) bool { return a.value == b.value },
			func(i item) string { return i.source },
			func(kept, dup item) { duplicates = append(duplicates, [2]item{kept, dup}) })
	}

	BeforeEach(func() {
		duplicates = nil
	})

	It("returns an empty result for no candidates", func() {
		result, conflicts := dedupe()
		Expect(result).To(BeEmpty())
		Expect(conflicts).To(BeEmpty())
	})

	It("keeps distinct keys in order", func() {
		result, conflicts := dedupe(item{"a", "1", "s1"}, item{"b", "2", "s2"})
		Expect(result).To(Equal([]item{{"a", "1", "s1"}, {"b", "2", "s2"}}))
		Expect(conflicts).To(BeEmpty())
		Expect(duplicates).To(BeEmpty())
	})

	It("keeps the first of identical candidates and reports the duplicate", func() {
		result, conflicts := dedupe(item{"a", "1", "s1"}, item{"a", "1", "s2"})
		Expect(result).To(Equal([]item{{"a", "1", "s1"}}))
		Expect(conflicts).To(BeEmpty())
		Expect(duplicates).To(Equal([][2]item{{{"a", "1", "s1"}, {"a", "1", "s2"}}}))
	})

	It("reports a conflict listing all sources of the key", func() {
		_, conflicts := dedupe(item{"a", "1", "s1"}, item{"a", "2", "s2"}, item{"a", "1", "s3"})
		Expect(conflicts).To(Equal([]Conflict{{Key: "a", Sources: []string{"s1", "s2", "s3"}}}))
	})

	It("returns conflicts in the order of their first occurrence", func() {
		_, conflicts := dedupe(
			item{"b", "1", "s1"}, item{"a", "1", "s2"},
			item{"a", "2", "s3"}, item{"b", "2", "s4"})
		Expect(conflicts).To(HaveLen(2))
		Expect(conflicts[0].Key).To(Equal("a"))
		Expect(conflicts[1].Key).To(Equal("b"))
	})
})
