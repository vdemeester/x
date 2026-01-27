package lazypr

import (
	"testing"
)

func TestPRDetail_HasConflicts(t *testing.T) {
	tests := []struct {
		name      string
		mergeable string
		want      bool
	}{
		{"conflicting", "CONFLICTING", true},
		{"mergeable", "MERGEABLE", false},
		{"unknown", "UNKNOWN", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PRDetail{Mergeable: tt.mergeable}
			if got := pr.HasConflicts(); got != tt.want {
				t.Errorf("HasConflicts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPRDetail_HasBuildFailure(t *testing.T) {
	tests := []struct {
		name        string
		statusState string
		want        bool
	}{
		{"failure", "FAILURE", true},
		{"error", "ERROR", true},
		{"success", "SUCCESS", false},
		{"pending", "PENDING", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PRDetail{StatusState: tt.statusState}
			if got := pr.HasBuildFailure(); got != tt.want {
				t.Errorf("HasBuildFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPRDetail_NeedsAttention(t *testing.T) {
	tests := []struct {
		name        string
		mergeable   string
		statusState string
		want        bool
	}{
		{"clean", "MERGEABLE", "SUCCESS", false},
		{"conflicts", "CONFLICTING", "SUCCESS", true},
		{"build failure", "MERGEABLE", "FAILURE", true},
		{"both", "CONFLICTING", "FAILURE", true},
		{"pending is ok", "MERGEABLE", "PENDING", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PRDetail{Mergeable: tt.mergeable, StatusState: tt.statusState}
			if got := pr.NeedsAttention(); got != tt.want {
				t.Errorf("NeedsAttention() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPRDetail_StatusIcon(t *testing.T) {
	tests := []struct {
		name        string
		state       string
		mergeable   string
		statusState string
		want        string
	}{
		{"success", "OPEN", "MERGEABLE", "SUCCESS", IconSuccess},
		{"failure", "OPEN", "MERGEABLE", "FAILURE", IconFailure},
		{"error", "OPEN", "MERGEABLE", "ERROR", IconFailure},
		{"pending", "OPEN", "MERGEABLE", "PENDING", IconPending},
		{"unknown", "OPEN", "MERGEABLE", "", IconUnknown},
		{"conflicts override", "OPEN", "CONFLICTING", "SUCCESS", IconConflict},
		{"merged", "MERGED", "UNKNOWN", "SUCCESS", IconMerged},
		{"closed", "CLOSED", "UNKNOWN", "", IconClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PRDetail{State: tt.state, Mergeable: tt.mergeable, StatusState: tt.statusState}
			if got := pr.StatusIcon(); got != tt.want {
				t.Errorf("StatusIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPRDetail_MergeableIcon(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		mergeable string
		want      string
	}{
		{"mergeable", "OPEN", "MERGEABLE", IconSuccess},
		{"conflicting", "OPEN", "CONFLICTING", IconFailure},
		{"unknown", "OPEN", "UNKNOWN", IconUnknown},
		{"empty", "OPEN", "", IconUnknown},
		{"merged", "MERGED", "UNKNOWN", IconMerged},
		{"closed", "CLOSED", "UNKNOWN", IconClosed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PRDetail{State: tt.state, Mergeable: tt.mergeable}
			if got := pr.MergeableIcon(); got != tt.want {
				t.Errorf("MergeableIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPRDetail_MergeableText(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		mergeable string
		want      string
	}{
		{"mergeable", "OPEN", "MERGEABLE", "MERGEABLE"},
		{"conflicting", "OPEN", "CONFLICTING", "CONFLICTING"},
		{"unknown", "OPEN", "UNKNOWN", "CHECKING..."},
		{"merged", "MERGED", "UNKNOWN", "MERGED"},
		{"closed", "CLOSED", "UNKNOWN", "CLOSED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := PRDetail{State: tt.state, Mergeable: tt.mergeable}
			if got := pr.MergeableText(); got != tt.want {
				t.Errorf("MergeableText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckIcon(t *testing.T) {
	tests := []struct {
		name       string
		conclusion string
		status     string
		want       string
	}{
		{"success", "success", "", IconSuccess},
		{"SUCCESS", "SUCCESS", "", IconSuccess},
		{"failure", "failure", "", IconFailure},
		{"FAILURE", "FAILURE", "", IconFailure},
		{"skipped", "skipped", "", IconSkipped},
		{"cancelled", "cancelled", "", IconCancelled},
		{"in_progress", "", "in_progress", IconPending},
		{"queued", "", "queued", IconPending},
		{"unknown", "other", "", IconUnknown},
		{"no conclusion no status", "", "", IconUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckIcon(tt.conclusion, tt.status); got != tt.want {
				t.Errorf("CheckIcon(%q, %q) = %q, want %q", tt.conclusion, tt.status, got, tt.want)
			}
		})
	}
}

func TestPRDetail_StateHelpers(t *testing.T) {
	tests := []struct {
		state    string
		isMerged bool
		isClosed bool
		isOpen   bool
	}{
		{"MERGED", true, false, false},
		{"CLOSED", false, true, false},
		{"OPEN", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			pr := PRDetail{State: tt.state}
			if got := pr.IsMerged(); got != tt.isMerged {
				t.Errorf("IsMerged() = %v, want %v", got, tt.isMerged)
			}
			if got := pr.IsClosed(); got != tt.isClosed {
				t.Errorf("IsClosed() = %v, want %v", got, tt.isClosed)
			}
			if got := pr.IsOpen(); got != tt.isOpen {
				t.Errorf("IsOpen() = %v, want %v", got, tt.isOpen)
			}
		})
	}
}
