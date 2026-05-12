package spreading

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockResource implements SpreadableResource for testing
type mockResource struct {
	metav1.ObjectMeta
	generation                     int64
	observedGeneration             int64
	observedSubResourceGenerations map[string]int64
	healthy                        bool
}

func (m *mockResource) GetGeneration() int64 {
	return m.generation
}

func (m *mockResource) GetObservedGeneration() int64 {
	return m.observedGeneration
}

func (m *mockResource) GetObservedSubResourceGenerations() map[string]int64 {
	return m.observedSubResourceGenerations
}

func (m *mockResource) IsHealthy() bool {
	return m.healthy
}

var _ = Describe("Spreading", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("Manager", func() {
		Describe("NewManager", func() {
			It("should create a manager with given config", func() {
				config := Config{
					StartTime:      time.Now(),
					SpreadSeed:     "test-seed",
					SpreadPeriod:   5 * time.Minute,
					SpreadInterval: 180 * time.Minute,
					Enabled:        true,
				}

				manager := NewManager(config)

				Expect(manager).NotTo(BeNil())
				Expect(manager.Config).To(Equal(config))
			})
		})

		Describe("NewDefaultManager", func() {
			It("should create a manager with default configuration", func() {
				manager := NewDefaultManager()

				Expect(manager).NotTo(BeNil())
				Expect(manager.Config.Enabled).To(BeTrue())
				Expect(manager.Config.SpreadPeriod).To(Equal(5 * time.Minute))
				Expect(manager.Config.SpreadInterval).To(Equal(180 * time.Minute))
				Expect(manager.Config.SpreadSeed).NotTo(BeEmpty())
				Expect(manager.Config.StartTime).NotTo(BeZero())
			})

			It("should respect ENABLE_STARTUP_SPREADING=false", func() {
				_ = os.Setenv("ENABLE_STARTUP_SPREADING", "false")
				defer func() { _ = os.Unsetenv("ENABLE_STARTUP_SPREADING") }()

				manager := NewDefaultManager()

				Expect(manager.Config.Enabled).To(BeFalse())
			})

			It("should respect STARTUP_SPREAD_PERIOD_MINUTES", func() {
				_ = os.Setenv("STARTUP_SPREAD_PERIOD_MINUTES", "10")
				defer func() { _ = os.Unsetenv("STARTUP_SPREAD_PERIOD_MINUTES") }()

				manager := NewDefaultManager()

				Expect(manager.Config.SpreadPeriod).To(Equal(10 * time.Minute))
			})

			It("should respect SPREAD_INTERVAL_MINUTES", func() {
				_ = os.Setenv("SPREAD_INTERVAL_MINUTES", "120")
				defer func() { _ = os.Unsetenv("SPREAD_INTERVAL_MINUTES") }()

				manager := NewDefaultManager()

				Expect(manager.Config.SpreadInterval).To(Equal(120 * time.Minute))
			})
		})

		Describe("Spread", func() {
			var (
				manager  *Manager
				resource *mockResource
			)

			BeforeEach(func() {
				manager = NewManager(Config{
					StartTime:      time.Now(),
					SpreadSeed:     "test-seed",
					SpreadPeriod:   5 * time.Minute,
					SpreadInterval: 180 * time.Minute,
					Enabled:        true,
				})

				resource = &mockResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-resource",
						Namespace: "test-namespace",
					},
					generation:         1,
					observedGeneration: 1,
					healthy:            true,
				}
			})

			Context("when spreading is disabled", func() {
				BeforeEach(func() {
					manager.Config.Enabled = false
				})

				It("should not spread", func() {
					err := manager.Spread(ctx, resource, nil)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when manager is nil", func() {
				It("should not spread", func() {
					var nilManager *Manager
					err := nilManager.Spread(ctx, resource, nil)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when outside spread period", func() {
				BeforeEach(func() {
					manager.Config.StartTime = time.Now().Add(-10 * time.Minute)
					manager.Config.SpreadPeriod = 5 * time.Minute
				})

				It("should not spread", func() {
					err := manager.Spread(ctx, resource, nil)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when generation has changed", func() {
				BeforeEach(func() {
					resource.generation = 2
					resource.observedGeneration = 1
				})

				It("should not spread", func() {
					err := manager.Spread(ctx, resource, nil)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when resource is unhealthy", func() {
				BeforeEach(func() {
					resource.healthy = false
				})

				It("should not spread", func() {
					err := manager.Spread(ctx, resource, nil)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when resource is being deleted", func() {
				BeforeEach(func() {
					now := metav1.Now()
					resource.DeletionTimestamp = &now
				})

				It("should not spread", func() {
					err := manager.Spread(ctx, resource, nil)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when subresource generations changed", func() {
				Context("with current subresources different from observed", func() {
					BeforeEach(func() {
						resource.observedSubResourceGenerations = map[string]int64{
							"preset1": 1,
							"preset2": 2,
						}
					})

					It("should not spread when current has different generation for same subresource", func() {
						currentSubResourceGenerations := map[string]int64{
							"preset1": 2, // changed
							"preset2": 2,
						}
						err := manager.Spread(ctx, resource, currentSubResourceGenerations)
						Expect(err).ToNot(HaveOccurred())
					})

					It("should not spread when current has additional subresources", func() {
						currentSubResourceGenerations := map[string]int64{
							"preset1": 1,
							"preset2": 2,
							"preset3": 1, // new subresource
						}
						err := manager.Spread(ctx, resource, currentSubResourceGenerations)
						Expect(err).ToNot(HaveOccurred())
					})

					It("should not spread when current has fewer subresources", func() {
						currentSubResourceGenerations := map[string]int64{
							"preset1": 1,
							// preset2 missing
						}
						err := manager.Spread(ctx, resource, currentSubResourceGenerations)
						Expect(err).ToNot(HaveOccurred())
					})

					It("should not spread when current subresource is missing from observed", func() {
						currentSubResourceGenerations := map[string]int64{
							"preset1": 1,
							"preset2": 2,
							"preset3": 1,
						}
						resource.observedSubResourceGenerations = map[string]int64{
							"preset1": 1,
							"preset2": 2,
						}
						err := manager.Spread(ctx, resource, currentSubResourceGenerations)
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("with nil and empty maps", func() {
					It("should spread when both current and observed are nil", func() {
						resource.observedSubResourceGenerations = nil
						err := manager.Spread(ctx, resource, nil)
						Expect(err).To(HaveOccurred())
						var spreadErr *RequiresSpreadError
						Expect(err).To(BeAssignableToTypeOf(spreadErr))
					})

					It("should spread when both current and observed are empty maps", func() {
						resource.observedSubResourceGenerations = map[string]int64{}
						err := manager.Spread(ctx, resource, map[string]int64{})
						Expect(err).To(HaveOccurred())
						var spreadErr *RequiresSpreadError
						Expect(err).To(BeAssignableToTypeOf(spreadErr))
					})

					It("should not spread when current is nil but observed is not", func() {
						resource.observedSubResourceGenerations = map[string]int64{
							"preset1": 1,
						}
						err := manager.Spread(ctx, resource, nil)
						Expect(err).ToNot(HaveOccurred())
					})

					It("should not spread when observed is nil but current is not", func() {
						resource.observedSubResourceGenerations = nil
						currentSubResourceGenerations := map[string]int64{
							"preset1": 1,
						}
						err := manager.Spread(ctx, resource, currentSubResourceGenerations)
						Expect(err).ToNot(HaveOccurred())
					})
				})

				Context("with matching subresource generations", func() {
					It("should spread when current and observed match exactly", func() {
						resource.observedSubResourceGenerations = map[string]int64{
							"preset1": 1,
							"preset2": 2,
							"preset3": 5,
						}
						currentSubResourceGenerations := map[string]int64{
							"preset1": 1,
							"preset2": 2,
							"preset3": 5,
						}
						err := manager.Spread(ctx, resource, currentSubResourceGenerations)
						Expect(err).To(HaveOccurred())
						var spreadErr *RequiresSpreadError
						Expect(err).To(BeAssignableToTypeOf(spreadErr))
					})

					It("should spread when single subresource matches", func() {
						resource.observedSubResourceGenerations = map[string]int64{
							"preset1": 42,
						}
						currentSubResourceGenerations := map[string]int64{
							"preset1": 42,
						}
						err := manager.Spread(ctx, resource, currentSubResourceGenerations)
						Expect(err).To(HaveOccurred())
						var spreadErr *RequiresSpreadError
						Expect(err).To(BeAssignableToTypeOf(spreadErr))
					})
				})
			})

			Context("when all conditions for spreading are met", func() {
				It("should return RequiresSpreadError with delay", func() {
					err := manager.Spread(ctx, resource, nil)
					Expect(err).To(HaveOccurred())

					var spreadErr *RequiresSpreadError
					Expect(err).To(BeAssignableToTypeOf(spreadErr))
					spreadErr = err.(*RequiresSpreadError)
					Expect(spreadErr.RequeueAfter).To(BeNumerically(">", 0))
				})
			})
		})

		Describe("calculateSpreadDelay", func() {
			var (
				manager *Manager
			)

			BeforeEach(func() {
				manager = NewManager(Config{
					StartTime:      time.Now(),
					SpreadSeed:     "test-seed",
					SpreadPeriod:   5 * time.Minute,
					SpreadInterval: 180 * time.Minute,
					Enabled:        true,
				})
			})

			It("should return delays within the valid bounds", func() {
				delay := manager.calculateSpreadDelay()

				// Delay should be non-negative and not exceed SpreadPeriod + SpreadInterval
				// (the maximum possible delay is to the end of the spreading window)
				maxDelay := manager.Config.SpreadPeriod + manager.Config.SpreadInterval
				Expect(delay).To(BeNumerically(">=", 0))
				Expect(delay).To(BeNumerically("<=", maxDelay))
			})

			It("should return delays that target times within the spread window", func() {
				delay := manager.calculateSpreadDelay()
				targetTime := time.Now().Add(delay)

				// Target time should fall within the spread window
				spreadWindowStart := manager.Config.StartTime.Add(manager.Config.SpreadPeriod)
				spreadWindowEnd := spreadWindowStart.Add(manager.Config.SpreadInterval)

				Expect(targetTime).To(BeTemporally(">=", spreadWindowStart))
				Expect(targetTime).To(BeTemporally("<=", spreadWindowEnd))
			})

			It("should consistently return delays within bounds across multiple calls", func() {
				// Call multiple times to verify all results are valid
				maxDelay := manager.Config.SpreadPeriod + manager.Config.SpreadInterval
				for i := range 100 {
					delay := manager.calculateSpreadDelay()

					Expect(delay).To(BeNumerically(">=", 0), "iteration %d", i)
					Expect(delay).To(BeNumerically("<=", maxDelay), "iteration %d", i)
				}
			})

			It("should return zero delay when target time has passed", func() {
				// Set start time far in the past so any random offset results in a past time
				manager.Config.StartTime = time.Now().Add(-200 * time.Minute)

				delay := manager.calculateSpreadDelay()

				Expect(delay).To(Equal(time.Duration(0)))
			})

			It("should return zero delay when spread interval has expired", func() {
				// Set start time to exactly (SpreadPeriod + SpreadInterval) ago
				manager.Config.StartTime = time.Now().Add(-(manager.Config.SpreadPeriod + manager.Config.SpreadInterval + time.Minute))

				delay := manager.calculateSpreadDelay()

				Expect(delay).To(Equal(time.Duration(0)))
			})

			It("should handle edge case at start of spread window", func() {
				// Set start time such that we're exactly at the beginning of the spread period
				manager.Config.StartTime = time.Now().Add(-manager.Config.SpreadPeriod)

				delay := manager.calculateSpreadDelay()

				// Should return a delay within the spread interval (0 to SpreadInterval since we're at the start)
				Expect(delay).To(BeNumerically(">=", 0))
				Expect(delay).To(BeNumerically("<=", manager.Config.SpreadInterval))
			})

			It("should handle small spread intervals correctly", func() {
				manager.Config.SpreadInterval = 1 * time.Minute
				manager.Config.StartTime = time.Now()

				delay := manager.calculateSpreadDelay()

				// Should return a delay within SpreadPeriod + SpreadInterval
				maxDelay := manager.Config.SpreadPeriod + manager.Config.SpreadInterval
				Expect(delay).To(BeNumerically(">=", 0))
				Expect(delay).To(BeNumerically("<=", maxDelay))
			})

			It("should handle large spread intervals correctly", func() {
				manager.Config.SpreadInterval = 1440 * time.Minute // 24 hours
				manager.Config.StartTime = time.Now()

				delay := manager.calculateSpreadDelay()

				// Should return a delay within SpreadPeriod + SpreadInterval
				maxDelay := manager.Config.SpreadPeriod + manager.Config.SpreadInterval
				Expect(delay).To(BeNumerically(">=", 0))
				Expect(delay).To(BeNumerically("<=", maxDelay))
			})
		})
	})

	Describe("RequiresSpreadError", func() {
		It("should implement error interface", func() {
			err := &RequiresSpreadError{RequeueAfter: 5 * time.Minute}
			Expect(err.Error()).NotTo(BeEmpty())
		})

		It("should be identifiable with errors.Is", func() {
			err1 := &RequiresSpreadError{RequeueAfter: 5 * time.Minute}
			err2 := &RequiresSpreadError{RequeueAfter: 10 * time.Minute}

			Expect(err1.Is(err2)).To(BeTrue())
		})
	})

	Describe("Helper functions", func() {
		Describe("subResourcesChanged", func() {
			Context("with nil inputs", func() {
				It("should return false when both maps are nil", func() {
					result := subResourcesChanged(nil, nil)
					Expect(result).To(BeFalse())
				})

				It("should return true when first map is nil and second is not", func() {
					result := subResourcesChanged(nil, map[string]int64{"key": 1})
					Expect(result).To(BeTrue())
				})

				It("should return true when second map is nil and first is not", func() {
					result := subResourcesChanged(map[string]int64{"key": 1}, nil)
					Expect(result).To(BeTrue())
				})
			})

			Context("with empty maps", func() {
				It("should return false when both maps are empty", func() {
					result := subResourcesChanged(map[string]int64{}, map[string]int64{})
					Expect(result).To(BeFalse())
				})

				It("should return true when first map is empty and second is not", func() {
					result := subResourcesChanged(map[string]int64{}, map[string]int64{"key": 1})
					Expect(result).To(BeTrue())
				})

				It("should return true when second map is empty and first is not", func() {
					result := subResourcesChanged(map[string]int64{"key": 1}, map[string]int64{})
					Expect(result).To(BeTrue())
				})
			})

			Context("with matching maps", func() {
				It("should return false when single entry matches", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 1},
						map[string]int64{"key1": 1},
					)
					Expect(result).To(BeFalse())
				})

				It("should return false when multiple entries match", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 1, "key2": 2, "key3": 3},
						map[string]int64{"key1": 1, "key2": 2, "key3": 3},
					)
					Expect(result).To(BeFalse())
				})

				It("should return false when entries match in different order", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 1, "key2": 2},
						map[string]int64{"key2": 2, "key1": 1},
					)
					Expect(result).To(BeFalse())
				})
			})

			Context("with different maps", func() {
				It("should return true when generation value differs", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 1},
						map[string]int64{"key1": 2},
					)
					Expect(result).To(BeTrue())
				})

				It("should return true when first map has extra key", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 1, "key2": 2},
						map[string]int64{"key1": 1},
					)
					Expect(result).To(BeTrue())
				})

				It("should return true when second map has extra key", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 1},
						map[string]int64{"key1": 1, "key2": 2},
					)
					Expect(result).To(BeTrue())
				})

				It("should return true when keys are completely different", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 1, "key2": 2},
						map[string]int64{"key3": 3, "key4": 4},
					)
					Expect(result).To(BeTrue())
				})

				It("should return true when one value differs in multi-entry maps", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 1, "key2": 2, "key3": 3},
						map[string]int64{"key1": 1, "key2": 99, "key3": 3},
					)
					Expect(result).To(BeTrue())
				})
			})

			Context("with edge cases", func() {
				It("should handle zero generation values correctly", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 0},
						map[string]int64{"key1": 0},
					)
					Expect(result).To(BeFalse())
				})

				It("should handle large generation values correctly", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 9223372036854775807},
						map[string]int64{"key1": 9223372036854775807},
					)
					Expect(result).To(BeFalse())
				})

				It("should detect difference between zero and non-zero generations", func() {
					result := subResourcesChanged(
						map[string]int64{"key1": 0},
						map[string]int64{"key1": 1},
					)
					Expect(result).To(BeTrue())
				})
			})
		})

		Describe("hashString", func() {
			It("should return consistent hash for same string", func() {
				hash1 := hashString("test-string")
				hash2 := hashString("test-string")

				Expect(hash1).To(Equal(hash2))
			})

			It("should return different hashes for different strings", func() {
				hash1 := hashString("test-string-1")
				hash2 := hashString("test-string-2")

				Expect(hash1).NotTo(Equal(hash2))
			})
		})

		Describe("getBoolEnv", func() {
			AfterEach(func() {
				_ = os.Unsetenv("TEST_BOOL")
			})

			It("should return default when env var is not set", func() {
				result := getBoolEnv("TEST_BOOL", true)
				Expect(result).To(BeTrue())
			})

			It("should parse 'true' correctly", func() {
				_ = os.Setenv("TEST_BOOL", "true")
				result := getBoolEnv("TEST_BOOL", false)
				Expect(result).To(BeTrue())
			})

			It("should parse 'false' correctly", func() {
				_ = os.Setenv("TEST_BOOL", "false")
				result := getBoolEnv("TEST_BOOL", true)
				Expect(result).To(BeFalse())
			})

			It("should return default on invalid value", func() {
				_ = os.Setenv("TEST_BOOL", "invalid")
				result := getBoolEnv("TEST_BOOL", true)
				Expect(result).To(BeTrue())
			})
		})

		Describe("getDurationEnv", func() {
			AfterEach(func() {
				_ = os.Unsetenv("TEST_DURATION")
			})

			It("should return default when env var is not set", func() {
				result := getDurationEnv("TEST_DURATION", 10)
				Expect(result).To(Equal(time.Duration(10)))
			})

			It("should parse valid duration correctly", func() {
				_ = os.Setenv("TEST_DURATION", "20")
				result := getDurationEnv("TEST_DURATION", 10)
				Expect(result).To(Equal(time.Duration(20)))
			})

			It("should return default on invalid value", func() {
				_ = os.Setenv("TEST_DURATION", "invalid")
				result := getDurationEnv("TEST_DURATION", 10)
				Expect(result).To(Equal(time.Duration(10)))
			})
		})
	})
})
