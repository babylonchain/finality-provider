package clientcontroller

import (
	"fmt"
	"testing"

	"github.com/babylonchain/babylon/x/finality/types"
	"github.com/stretchr/testify/require"
)

func TestExpectedErr(t *testing.T) {
	expectedErr := Expected(types.ErrDuplicatedFinalitySig)
	require.True(t, IsExpected(expectedErr))
	wrappedErr := fmt.Errorf("expected: %w", expectedErr)
	require.True(t, IsExpected(wrappedErr))
}
