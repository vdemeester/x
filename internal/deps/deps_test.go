package deps

import (
	"reflect"
	"testing"
)

func TestDependencies_HasPackage(t *testing.T) {
	deps := &Dependencies{
		Packages: []Package{
			{Name: "git"},
			{Name: "OpenSSH"}, // Test case sensitivity
			{Name: "curl"},
		},
	}

	tests := []struct {
		name    string
		pkgName string
		want    bool
	}{
		{
			name:    "exact match lowercase",
			pkgName: "git",
			want:    true,
		},
		{
			name:    "case insensitive match",
			pkgName: "openssh",
			want:    true,
		},
		{
			name:    "case insensitive uppercase",
			pkgName: "CURL",
			want:    true,
		},
		{
			name:    "non-existent package",
			pkgName: "nonexistent",
			want:    false,
		},
		{
			name:    "empty string",
			pkgName: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deps.HasPackage(tt.pkgName)
			if got != tt.want {
				t.Errorf("HasPackage(%q) = %v, want %v", tt.pkgName, got, tt.want)
			}
		})
	}
}

func TestDependencies_HasModulePath(t *testing.T) {
	deps := &Dependencies{
		Modules: []ModulePath{
			{Path: "nixos/modules/services/networking/ssh/sshd.nix", Type: "nixos"},
			{Path: "home-manager/modules/programs/git.nix", Type: "home-manager"},
		},
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing nixos module",
			path: "nixos/modules/services/networking/ssh/sshd.nix",
			want: true,
		},
		{
			name: "existing home-manager module",
			path: "home-manager/modules/programs/git.nix",
			want: true,
		},
		{
			name: "non-existent module",
			path: "nixos/modules/services/web/nginx.nix",
			want: false,
		},
		{
			name: "empty path",
			path: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deps.HasModulePath(tt.path)
			if got != tt.want {
				t.Errorf("HasModulePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDeduplicatePackages(t *testing.T) {
	tests := []struct {
		name  string
		input []Package
		want  []Package
	}{
		{
			name:  "no duplicates",
			input: []Package{{Name: "git"}, {Name: "openssh"}},
			want:  []Package{{Name: "git"}, {Name: "openssh"}},
		},
		{
			name:  "exact duplicates",
			input: []Package{{Name: "git"}, {Name: "git"}, {Name: "openssh"}},
			want:  []Package{{Name: "git"}, {Name: "openssh"}},
		},
		{
			name:  "multiple duplicates",
			input: []Package{{Name: "git"}, {Name: "git"}, {Name: "openssh"}, {Name: "openssh"}, {Name: "curl"}},
			want:  []Package{{Name: "git"}, {Name: "openssh"}, {Name: "curl"}},
		},
		{
			name:  "empty input",
			input: []Package{},
			want:  []Package{},
		},
		{
			name:  "single package",
			input: []Package{{Name: "git"}},
			want:  []Package{{Name: "git"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicatePackages(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("deduplicatePackages() got %d packages, want %d", len(got), len(tt.want))
				return
			}

			// Build maps for comparison (order doesn't matter for duplicates)
			gotMap := make(map[string]bool)
			wantMap := make(map[string]bool)
			for _, p := range got {
				gotMap[p.Name] = true
			}
			for _, p := range tt.want {
				wantMap[p.Name] = true
			}

			if !reflect.DeepEqual(gotMap, wantMap) {
				t.Errorf("deduplicatePackages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		hostDeps map[string]*Dependencies
		want     *Dependencies
	}{
		{
			name: "single host",
			hostDeps: map[string]*Dependencies{
				"host1": {
					Packages: []Package{{Name: "git"}, {Name: "openssh"}},
					Modules:  []ModulePath{{Path: "nixos/modules/services/ssh.nix", Type: "nixos"}},
					Services: []string{"sshd"},
				},
			},
			want: &Dependencies{
				Packages: []Package{{Name: "git"}, {Name: "openssh"}},
				Modules:  []ModulePath{{Path: "nixos/modules/services/ssh.nix", Type: "nixos"}},
				Services: []string{"sshd"},
			},
		},
		{
			name: "multiple hosts with overlap",
			hostDeps: map[string]*Dependencies{
				"host1": {
					Packages: []Package{{Name: "git"}, {Name: "openssh"}},
					Modules:  []ModulePath{{Path: "nixos/modules/services/ssh.nix", Type: "nixos"}},
					Services: []string{"sshd"},
				},
				"host2": {
					Packages: []Package{{Name: "git"}, {Name: "curl"}},
					Modules:  []ModulePath{{Path: "nixos/modules/programs/git.nix", Type: "nixos"}},
					Services: []string{"nginx"},
				},
			},
			want: &Dependencies{
				Packages: []Package{{Name: "git"}, {Name: "openssh"}, {Name: "curl"}},
				Modules: []ModulePath{
					{Path: "nixos/modules/services/ssh.nix", Type: "nixos"},
					{Path: "nixos/modules/programs/git.nix", Type: "nixos"},
				},
				Services: []string{"sshd", "nginx"},
			},
		},
		{
			name: "multiple hosts all duplicates",
			hostDeps: map[string]*Dependencies{
				"host1": {
					Packages: []Package{{Name: "git"}},
				},
				"host2": {
					Packages: []Package{{Name: "git"}},
				},
			},
			want: &Dependencies{
				Packages: []Package{{Name: "git"}},
				Modules:  []ModulePath{},
				Services: []string{},
			},
		},
		{
			name:     "empty input",
			hostDeps: map[string]*Dependencies{},
			want: &Dependencies{
				Packages: []Package{},
				Modules:  []ModulePath{},
				Services: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Merge(tt.hostDeps)

			// Check packages count
			if len(got.Packages) != len(tt.want.Packages) {
				t.Errorf("Merge() got %d packages, want %d", len(got.Packages), len(tt.want.Packages))
			}

			// Check modules count
			if len(got.Modules) != len(tt.want.Modules) {
				t.Errorf("Merge() got %d modules, want %d", len(got.Modules), len(tt.want.Modules))
			}

			// Check services count
			if len(got.Services) != len(tt.want.Services) {
				t.Errorf("Merge() got %d services, want %d", len(got.Services), len(tt.want.Services))
			}

			// Verify all expected packages exist
			for _, wantPkg := range tt.want.Packages {
				if !got.HasPackage(wantPkg.Name) {
					t.Errorf("Merge() missing expected package %q", wantPkg.Name)
				}
			}

			// Verify all expected modules exist
			for _, wantMod := range tt.want.Modules {
				if !got.HasModulePath(wantMod.Path) {
					t.Errorf("Merge() missing expected module %q", wantMod.Path)
				}
			}
		})
	}
}
