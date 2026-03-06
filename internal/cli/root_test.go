package cli

import "testing"

func TestRunOptionsFromFlagsRejectsMixedProcessAndGroup(t *testing.T) {
	t.Parallel()

	if _, err := runOptionsFromFlags("api", "dev", false); err == nil {
		t.Fatal("expected error when both process and group are set")
	}
}
