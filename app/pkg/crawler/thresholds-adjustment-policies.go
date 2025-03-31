package crawler

import (
	"fmt"

	"crawler/app/pkg/assert"
	assetshandler "crawler/app/pkg/assets-handler"
	"crawler/app/pkg/thresholds"

	"github.com/expr-lang/expr"
)

type policyExprParams struct {
	CurrentTimestamp uint32
	NewTimestamp     uint32
	ThresholdsAmount uint16
}

func compilePolicies(
	policiesCfgs []assetshandler.ThresholdsAdjPolicyCfg,
) ([]*thresholds.ThresholdsAdjustmentPolicy, error) {
	policies := make([]*thresholds.ThresholdsAdjustmentPolicy, len(policiesCfgs))

	for idx, policyCfg := range policiesCfgs {
		compiledExpr, err := expr.Compile(
			policyCfg.ComputeIncrementExpr,
			expr.Env(policyExprParams{}),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"error compiling thresholds adjustment policy with percentage %.2f: %w",
				policyCfg.Percentage, err,
			)
		}

		policies[idx] = &thresholds.ThresholdsAdjustmentPolicy{
			Percentage: policyCfg.Percentage,
			ComputeIncrement: func(currentTimestamp, newTimestamp uint32, thresholdsAmount uint16) int32 {
				params := policyExprParams{
					CurrentTimestamp: currentTimestamp,
					NewTimestamp:     newTimestamp,
					ThresholdsAmount: thresholdsAmount,
				}

				result, err := expr.Run(compiledExpr, params)
				assert.NoError(
					err,
					"error running successfully compiled thresholds adjustment expression, "+
						"probably a bad parameter has been passed",
					assert.AssertData{
						"policyPercentage": policyCfg.Percentage,
						"currentTimestamp": currentTimestamp,
						"newTimestamp":     newTimestamp,
						"thresholdsAmount": thresholdsAmount,
					},
				)

				switch r := result.(type) {
				case float64:
					return int32(r)
				case int:
					return int32(r)
				default:
					assert.Never("a non float64/int value has been returned by a "+
						"successfully compiled thresholds adjustment expression",
						assert.AssertData{
							"policyPercentage": policyCfg.Percentage,
							"currentTimestamp": currentTimestamp,
							"newTimestamp":     newTimestamp,
							"thresholdsAmount": thresholdsAmount,
							"result":           result,
						},
					)
					return 0 // unreachable but needed to compile
				}
			},
		}
	}

	return policies, nil
}
