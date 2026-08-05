package waf

import "testing"

func TestDeepDecodeLayers(t *testing.T) {
	d := NewDecoder(DefaultConfig().Decoding)

	cases := []struct {
		name    string
		in      string
		wantSub string // decoded form must contain this
	}{
		{"url-encoded", `%3Cscript%3E`, "<script>"},
		{"double-url-encoded", `%253Cscript%253E`, "<script>"},
		{"html-named-entity", `&lt;script&gt;`, "<script>"},
		{"html-numeric-entity", `&#60;script&#62;`, "<script>"},
		{"html-hex-entity", `&#x3c;script&#x3e;`, "<script>"},
		{"unicode-escape", `\u003cscript\u003e`, "<script>"},
		{"unicode-x-escape", `\x3cscript\x3e`, "<script>"},
		{"sql-comment-split", `un/**/ion`, "union"},
		{"whitespace-collapse", "union    select", "union select"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := d.DeepDecode(c.in)
			if !contains(got, c.wantSub) {
				t.Errorf("DeepDecode(%q) = %q, want substring %q", c.in, got, c.wantSub)
			}
		})
	}
}

func TestDecodeDepthBounded(t *testing.T) {
	cfg := DefaultConfig().Decoding
	cfg.MaxDecodeDepth = 2
	d := NewDecoder(cfg)
	// Triple-encoded; with depth 2 it should not fully resolve, proving the
	// bound is enforced (a decode bomb cannot force unbounded work).
	in := `%25253Cscript%25253E`
	got := d.DeepDecode(in)
	if got == "<script>" {
		t.Errorf("expected bounded decode to stop before full resolution, got %q", got)
	}
}

func TestVariantsIncludesRawAndDecoded(t *testing.T) {
	d := NewDecoder(DefaultConfig().Decoding)
	vs := d.Variants(`%3Cscript%3E`)
	if len(vs) != 2 {
		t.Fatalf("expected raw+decoded variants, got %d: %v", len(vs), vs)
	}
	if vs[0] != `%3Cscript%3E` {
		t.Errorf("first variant should be raw, got %q", vs[0])
	}
}

func TestBase64OnlyDecodesPlausibleTokens(t *testing.T) {
	d := NewDecoder(DefaultConfig().Decoding)
	// Ordinary text must not be mangled by an over-eager base64 pass.
	if got := d.DeepDecode("hello world this is fine"); got != "hello world this is fine" {
		t.Errorf("benign text altered by decoder: %q", got)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
