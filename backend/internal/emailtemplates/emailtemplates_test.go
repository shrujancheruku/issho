package emailtemplates

import (
	htmltemplate "html/template"
	"strings"
	"testing"
	texttemplate "text/template"
)

var src = Source{
	Languages: []string{"ja", "ar"},
	Strings: map[string]string{
		"subject": "Your Issho sign-in code",
		"heading": "Your Issho sign-in code",
		"expiry":  "This code expires in 10 minutes.",
		"ignore":  "If you didn't ask for this code, you can ignore this email.",
	},
}

var tr = map[string]Text{
	"ja": {"subject": "Issho のログインコード", "heading": "Issho のログインコード", "expiry": "このコードの有効期限は10分です。", "ignore": "心当たりがない場合は無視してください。"},
	"ar": {"subject": "رمز تسجيل الدخول", "heading": "رمز تسجيل الدخول", "expiry": "تنتهي صلاحية هذا الرمز خلال 10 دقائق.", "ignore": "تجاهل هذا البريد & <b>"},
}

// execute renders a generated template the way Supabase Auth does: Go
// templates with .Token and .Data (user_metadata). Both template packages, as
// either could be in use.
func execute(t *testing.T, tmpl string, data map[string]any) []string {
	t.Helper()
	vars := map[string]any{"Token": "123456", "Data": data}
	var outs []string
	tt, err := texttemplate.New("t").Parse(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	ht, err := htmltemplate.New("h").Parse(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	for _, exec := range []func(*strings.Builder) error{
		func(b *strings.Builder) error { return tt.Execute(b, vars) },
		func(b *strings.Builder) error { return ht.Execute(b, vars) },
	} {
		var b strings.Builder
		if err := exec(&b); err != nil {
			t.Fatalf("data %v: %v", data, err)
		}
		outs = append(outs, b.String())
	}
	return outs
}

func TestRender(t *testing.T) {
	body, subject, err := Render(src, tr)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		data    map[string]any
		want    string
		subject string
	}{
		{"japanese", map[string]any{"lang": "ja"}, "このコードの有効期限は10分です。", "Issho のログインコード"},
		{"unknown language", map[string]any{"lang": "sw"}, "This code expires in 10 minutes.", "Your Issho sign-in code"},
		{"no lang", map[string]any{"name": "x"}, "This code expires in 10 minutes.", "Your Issho sign-in code"},
		{"no metadata", nil, "This code expires in 10 minutes.", "Your Issho sign-in code"},
		{"lang not a string", map[string]any{"lang": 42}, "This code expires in 10 minutes.", "Your Issho sign-in code"},
	} {
		for _, out := range execute(t, body, tc.data) {
			if !strings.Contains(out, tc.want) || !strings.Contains(out, "123456") {
				t.Errorf("%s: body missing %q or the code:\n%s", tc.name, tc.want, out)
			}
			if strings.Count(out, "<div lang=") != 1 {
				t.Errorf("%s: exactly one language block expected:\n%s", tc.name, out)
			}
		}
		for _, out := range execute(t, subject, tc.data) {
			if strings.TrimSpace(out) != tc.subject {
				t.Errorf("%s: subject %q, want %q", tc.name, out, tc.subject)
			}
		}
	}
	// Right to left, and translations are escaped (the template text is ours).
	out := execute(t, body, map[string]any{"lang": "ar"})[0]
	if !strings.Contains(out, `lang="ar" dir="rtl"`) || !strings.Contains(out, "&amp; &lt;b&gt;") || !strings.Contains(out, `dir="ltr">123456`) {
		t.Errorf("arabic block:\n%s", out)
	}
}

func TestRenderRejects(t *testing.T) {
	for name, bad := range map[string]string{
		"template syntax": "{{ .Token }} です",
		"toml quotes":     "a ''' b",
		"newline":         "a\nb",
		"empty":           "  ",
	} {
		ja := Text{}
		for k, v := range tr["ja"] {
			ja[k] = v
		}
		ja["expiry"] = bad
		if _, _, err := Render(src, map[string]Text{"ja": ja, "ar": tr["ar"]}); err == nil {
			t.Errorf("%s: accepted %q", name, bad)
		}
	}
	if _, _, err := Render(src, map[string]Text{"ja": tr["ja"]}); err == nil {
		t.Error("a missing language must fail")
	}
}

func TestSetSubjects(t *testing.T) {
	config := "[auth.email.template.magic_link]\nsubject = \"old\"\ncontent_path = \"x\"\n\n[auth.email.template.confirmation]\nsubject = \"old\"\n"
	out, err := SetSubjects(config, "S")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "subject = '''S'''") != 2 || strings.Contains(out, `"old"`) || !strings.Contains(out, `content_path = "x"`) {
		t.Errorf("got:\n%s", out)
	}
	if _, err := SetSubjects("[auth]\n", "S"); err == nil {
		t.Error("a config without the template sections must fail")
	}
}

func TestReadSource(t *testing.T) {
	s, err := ReadSource("../../../supabase/templates/source.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Languages) == 0 {
		t.Error("no languages")
	}
}
