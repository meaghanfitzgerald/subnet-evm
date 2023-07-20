// (c) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package warp

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ava-labs/subnet-evm/params"
	"github.com/ava-labs/subnet-evm/precompile/precompileconfig"
	"github.com/stretchr/testify/require"
)

func TestVerifyWarpconfig(t *testing.T) {
	tests := []struct {
		name          string
		config        precompileconfig.Config
		ExpectedError string
	}{
		{
			name:          "quorum numerator less than minimum",
			config:        NewConfig(big.NewInt(3), params.WarpQuorumNumeratorMinimum-1),
			ExpectedError: fmt.Sprintf("cannot specify quorum numerator (%d) < min quorum numerator (%d)", params.WarpQuorumNumeratorMinimum-1, params.WarpQuorumNumeratorMinimum),
		},
		{
			name:          "quorum numerator > quorum denominator",
			config:        NewConfig(big.NewInt(3), params.WarpQuorumDenominator+1),
			ExpectedError: fmt.Sprintf("cannot specify quorum numerator (%d) > quorum denominator (%d)", params.WarpQuorumDenominator+1, params.WarpQuorumDenominator),
		},
		{
			name:   "default quorum numerator",
			config: NewDefaultConfig(big.NewInt(3)),
		},
		{
			name:   "valid quorum numerator 1 less than denominator",
			config: NewConfig(big.NewInt(3), params.WarpQuorumDenominator-1),
		},
		{
			name:   "valid quorum numerator 1 more than minimum",
			config: NewConfig(big.NewInt(3), params.WarpQuorumNumeratorMinimum+1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			err := tt.config.Verify()
			if tt.ExpectedError == "" {
				require.NoError(err)
			} else {
				require.ErrorContains(err, tt.ExpectedError)
			}
		})
	}
}

func TestEqualWarpConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   precompileconfig.Config
		other    precompileconfig.Config
		expected bool
	}{
		{
			name:     "non-nil config and nil other",
			config:   NewDefaultConfig(big.NewInt(3)),
			other:    nil,
			expected: false,
		},
		{
			name:     "different type",
			config:   NewDefaultConfig(big.NewInt(3)),
			other:    precompileconfig.NewNoopStatefulPrecompileConfig(),
			expected: false,
		},
		{
			name:     "different timestamp",
			config:   NewDefaultConfig(big.NewInt(3)),
			other:    NewDefaultConfig(big.NewInt(4)),
			expected: false,
		},
		{
			name:     "different quorum numerator",
			config:   NewConfig(big.NewInt(3), params.WarpQuorumNumeratorMinimum+1),
			other:    NewConfig(big.NewInt(3), params.WarpQuorumNumeratorMinimum+2),
			expected: false,
		},
		{
			name:     "same default config",
			config:   NewDefaultConfig(big.NewInt(3)),
			other:    NewDefaultConfig(big.NewInt(3)),
			expected: true,
		},
		{
			name:     "same non-default config",
			config:   NewConfig(big.NewInt(3), params.WarpQuorumNumeratorMinimum+5),
			other:    NewConfig(big.NewInt(3), params.WarpQuorumNumeratorMinimum+5),
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			require.Equal(tt.expected, tt.config.Equal(tt.other))
		})
	}
}
