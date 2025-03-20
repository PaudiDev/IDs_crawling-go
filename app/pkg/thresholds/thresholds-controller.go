package thresholds

import (
	"errors"
	"fmt"
	"math"
)

type ThresholdsController struct {
	// The state of the controller.
	state *thresholdsState

	// A set of configs that the controller will use in construction and to
	// adjust the thresholds amount whenever a ThresholdsControllerInput is received.
	cfg *ThresholdsControllerConfig
}

type thresholdsState struct {
	// The amount of IDs thresholds the controller is currently managing.
	//
	// Initialized with cfg.InitialThresholdsAmount.
	thresholdsAmount uint16

	// The timestamp of the last input received.
	// It is used to calculate the increment of the thresholds amount by comparing
	// it to the timestamp of the new input.
	//
	// Initialized with math.MaxUint32 (2^32 - 1).
	currentTimestamp uint32
}

type ThresholdsControllerConfig struct {
	// The amount of thresholds the controller will start with.
	// This value must be greater than 0.
	InitialThresholdsAmount uint16

	// When the controller receives an input (Update is called) all policies are iterated until match.
	// A match is reached when input.ThresholdLevel is higher than or equal to
	// policy.Percentage * thresholdsAmount.
	//
	// At this point thresholdsAmount is increased by
	// policy.ComputeIncrement(currentTimestamp, input.Timestamp).
	//
	// To decrease the amount of thresholds policy.ComputeIncrement
	// must return a negative value.
	//
	// At least one policy must be provided and present a percentage of 0.
	// The slice should be ordered by percentage in descending order to ensure correct behaviour.
	// All percentages must be in the range [0, 1] and should be different from each other.
	ThresholdsAdjustmentPolicies []ThresholdsAdjustmentPolicy
}

type ThresholdsAdjustmentPolicy struct {
	// The percentage of thresholdsAmount that must be less than or equal to
	// input.ThresholdLevel to trigger the policy.
	Percentage float32

	// The function that will be called to compute the increment of thresholdsAmount.
	// It receives the controller currentTimestamp, input.Timestamp and
	// the current thresholdsAmount.
	//
	// These 3 values can be used to calculate the increment based on the custom
	// policy logic. The passed thresholdsAmount can be used to return percentages
	// of it as the increment
	// ( e.g. return int32(float32(thresholdsAmount) * 0.25) ).
	ComputeIncrement func(currentTimestamp, newTimestamp uint32, thresholdsAmount uint16) int32
}

type ThresholdsControllerInput struct {
	// The level of the threshold that has been hit.
	ThresholdLevel uint16

	// The timestamp of the item that hit the threshold.
	Timestamp uint32
}

func NewThresholdsController(cfg *ThresholdsControllerConfig) (*ThresholdsController, error) {
	// Validate that the initial thresholds amount is greater than 0.
	if cfg.InitialThresholdsAmount == 0 {
		return nil, fmt.Errorf(
			"InitialThresholdsAmount must be greater than 0, %d has been provided",
			cfg.InitialThresholdsAmount,
		)
	}

	// Validate that at least one policy has been provided.
	if len(cfg.ThresholdsAdjustmentPolicies) == 0 {
		return nil, errors.New("no ThresholdsAdjustmentPolicy provided, at least one is required")
	}

	// Validate that all policies percentages are in the range [0, 1]
	// and at least one policy has a percentage of 0.
	foundZero := false
	for idx, policy := range cfg.ThresholdsAdjustmentPolicies {
		if policy.Percentage < 0 || policy.Percentage > 1 {
			return nil, fmt.Errorf(
				"policy percentages must be in the range [0, 1]."+
					"policy N. %d has a percentage of %f",
				idx, policy.Percentage,
			)
		}
		if !foundZero && policy.Percentage == 0 {
			foundZero = true
		}
	}
	if !foundZero {
		return nil, errors.New("at least one policy must have a percentage of 0")
	}

	thresholdsController := &ThresholdsController{
		cfg: cfg,
		state: &thresholdsState{
			thresholdsAmount: cfg.InitialThresholdsAmount,
			currentTimestamp: math.MaxUint32,
		},
	}

	return thresholdsController, nil
}

// When this function is called all policies are iterated until match.
// A match is reached when input.ThresholdLevel is higher than or equal to
// policy.Percentage * thresholdsAmount.
//
// At this point thresholdsAmount is increased by
// policy.ComputeIncrement(currentTimestamp, input.Timestamp, thresholdsAmount).
// CurrentTimestamp is then updated to input.Timestamp.
//
// See ThresholdsAdjustmentPolicy for more information about the policies.
func (tc *ThresholdsController) Update(input *ThresholdsControllerInput) {
	// This approach implements the strategy pattern, granting scalability and flexibility.
	for _, policy := range tc.cfg.ThresholdsAdjustmentPolicies {
		minMatchingLevel := policy.Percentage * float32(tc.state.thresholdsAmount)

		if input.ThresholdLevel >= uint16(math.Ceil(float64(minMatchingLevel))) {
			increment := policy.ComputeIncrement(
				tc.state.currentTimestamp, input.Timestamp, tc.state.thresholdsAmount,
			)

			if increment < 0 {
				tc.state.thresholdsAmount = uint16(
					math.Max(
						1, float64(tc.state.thresholdsAmount)+float64(increment),
					),
				)
			} else {
				tc.state.thresholdsAmount += uint16(increment)
			}

			break
		}
	}

	tc.state.currentTimestamp = input.Timestamp
}

// Return the current thresholds amount of the controller.
// This value is always greater than 0.
func (tc *ThresholdsController) GetThresholdsAmount() uint16 {
	return tc.state.thresholdsAmount
}

// Return the current timestamp of the controller.
func (tc *ThresholdsController) GetCurrentTimestamp() uint32 {
	return tc.state.currentTimestamp
}
