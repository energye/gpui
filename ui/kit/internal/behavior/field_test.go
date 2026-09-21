package behavior_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/behavior"
)

type fieldFile struct {
	Cases []struct {
		Name          string  `json:"name"`
		Value         *string `json:"value"`
		Default       string  `json:"default"`
		Input         string  `json:"input"`
		Composing     string  `json:"composing"`
		Commit        bool    `json:"commit"`
		Initial       string  `json:"initial"`
		WantValue     string  `json:"wantValue"`
		WantDisplay   string  `json:"wantDisplay"`
		WantCalls     int     `json:"wantCalls"`
		WantNotified  string  `json:"wantNotified"`
		WantComposing bool    `json:"wantComposing"`
		Validate      string  `json:"validate"`
		WantError     string  `json:"wantError"`
	} `json:"cases"`
}

func loadField(t *testing.T) fieldFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "field_cases.json"))
	if err != nil {
		t.Fatalf("read field_cases.json: %v", err)
	}
	var f fieldFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode field_cases.json: %v", err)
	}
	if len(f.Cases) == 0 {
		t.Fatal("field_cases.json holds no cases")
	}
	return f
}

func nonemptyValidate(s string) string {
	if s == "" || s == "bad" {
		return "required"
	}
	return ""
}

func TestField_ValueAndComposing(t *testing.T) {
	f := loadField(t)
	for _, c := range f.Cases {
		var notified []string
		cfg := behavior.FieldConfig{
			Value:        c.Value,
			DefaultValue: c.Default,
			OnChange:     func(s string) { notified = append(notified, s) },
		}
		if c.Validate == "nonempty" {
			cfg.Validate = nonemptyValidate
		}
		// Initial for composing cases rides on DefaultValue.
		if c.Initial != "" && c.Value == nil && c.Default == "" {
			cfg.DefaultValue = c.Initial
		}
		fld := behavior.NewField(cfg)
		if c.Composing != "" && !c.Commit {
			fld.SetComposing(c.Composing)
			if got := fld.Value(); got != c.WantValue {
				t.Fatalf("%s: value=%q want %q (composing must not count)", c.Name, got, c.WantValue)
			}
			if got := fld.Display(); got != c.WantDisplay {
				t.Fatalf("%s: display=%q want %q", c.Name, got, c.WantDisplay)
			}
			if got := fld.IsComposing(); got != c.WantComposing {
				t.Fatalf("%s: composing=%v want %v", c.Name, got, c.WantComposing)
			}
			if len(notified) != c.WantCalls {
				t.Fatalf("%s: calls=%d want %d (composing fires nothing)", c.Name, len(notified), c.WantCalls)
			}
			continue
		}
		if c.Composing != "" {
			fld.SetComposing(c.Composing)
			if !fld.CommitComposing() {
				t.Fatalf("%s: CommitComposing refused", c.Name)
			}
		} else if c.Input != "" || c.Name == "validate_passes" || c.Name == "validate_flags_error" {
			fld.Input(c.Input)
		}
		if got := fld.Value(); got != c.WantValue {
			t.Fatalf("%s: value=%q want %q", c.Name, got, c.WantValue)
		}
		if c.WantDisplay != "" {
			if got := fld.Display(); got != c.WantDisplay {
				t.Fatalf("%s: display=%q want %q", c.Name, got, c.WantDisplay)
			}
		}
		if len(notified) != c.WantCalls {
			t.Fatalf("%s: calls=%d want %d", c.Name, len(notified), c.WantCalls)
		}
		if c.WantCalls > 0 && notified[len(notified)-1] != c.WantNotified {
			t.Fatalf("%s: notified=%q want %q", c.Name, notified[len(notified)-1], c.WantNotified)
		}
		if c.Validate != "" {
			if got := fld.Validate(); got != c.WantError {
				t.Fatalf("%s: error=%q want %q", c.Name, got, c.WantError)
			}
			if fld.HasError() != (c.WantError != "") {
				t.Fatalf("%s: HasError mismatch", c.Name)
			}
		}
	}
}

func TestField_ControlledSync(t *testing.T) {
	v := "hello"
	var notified []string
	fld := behavior.NewField(behavior.FieldConfig{
		Value:    &v,
		OnChange: func(s string) { notified = append(notified, s) },
	})
	if !fld.IsControlled() {
		t.Fatal("must report controlled")
	}
	fld.Input("world")
	if fld.Value() != "hello" {
		t.Fatalf("controlled stored %q, must keep props value", fld.Value())
	}
	if len(notified) != 1 || notified[0] != "world" {
		t.Fatalf("controlled notify=%v", notified)
	}
	if !fld.SyncExternal("world") {
		t.Fatal("SyncExternal must apply props change")
	}
	if fld.Value() != "world" {
		t.Fatalf("synced value=%q want world", fld.Value())
	}
	if len(notified) != 1 {
		t.Fatal("SyncExternal must not notify")
	}
}
