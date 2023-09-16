package handlers

import (
	"fmt"
	"testing"

	"github.com/openfaas/faas-provider/types"
)

func Test_validateScalingLabels(t *testing.T) {

	testCases := []struct {
		Name   string
		Labels map[string]string
		Err    error
	}{
		{
			Name:   "empty labels",
			Labels: map[string]string{},
			Err:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			gotErr := validateScalingLabels(&types.FunctionDeployment{Labels: &tc.Labels})
			got := fmt.Errorf("")
			if gotErr != nil {
				got = gotErr
			}
			want := fmt.Errorf("")
			if tc.Err != nil {
				want = tc.Err
			}

			if got.Error() != want.Error() {
				t.Errorf("got: %v, want: %v", got, want)
			}
		})
	}

}
