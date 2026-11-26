package conformance_test

import (
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
)

func TestCodeUnitsComposeWTF8(t *testing.T) {
	for _, c := range []struct {
		units conformance.CodeUnits
		want  string
	}{
		{conformance.CodeUnits{}, ""},
		{conformance.CodeUnits{"0061", "00E9", "952E"}, "aé键"},
		// 成对的代理项合成平面以外的一个字符，编成 4 个字节
		{conformance.CodeUnits{"D83D", "DE00"}, "\U0001F600"},
		// 孤立的代理项照 UTF-8 三字节的写法编出，不换成 U+FFFD
		{conformance.CodeUnits{"0061", "D800"}, "a\xed\xa0\x80"},
		{conformance.CodeUnits{"DC00"}, "\xed\xb0\x80"},
		{conformance.CodeUnits{"DE00", "D83D"}, "\xed\xb8\x80\xed\xa0\xbd"},
		{conformance.CodeUnits{"D83D", "D83D", "DE00"}, "\xed\xa0\xbd\U0001F600"},
		{conformance.CodeUnits{"dbff", "DFFF"}, "\U0010FFFF"},
	} {
		got, err := c.units.Text()
		if err != nil || got != c.want {
			t.Errorf("%v: got %q, %v; want %q", c.units, got, err, c.want)
		}
	}
	for _, bad := range []conformance.CodeUnits{{"61"}, {"00061"}, {"G000"}, {"+061"}, {""}} {
		if _, err := bad.Text(); err == nil {
			t.Errorf("%v must be rejected: code units are four hexadecimal digits", bad)
		}
	}
}

func TestCaseInputTakesTheTextOrTheCodeUnits(t *testing.T) {
	text, err := conformance.BlobKeyCase{Name: "text", Key: "a/b"}.Input()
	if err != nil || text != "a/b" {
		t.Errorf("a case with a key gives %q, %v", text, err)
	}
	text, err = conformance.CacheKeyCase{Name: "units", UTF16: conformance.CodeUnits{"0061", "D800"}}.Input()
	if err != nil || text != "a\xed\xa0\x80" {
		t.Errorf("a case with code units gives %q, %v", text, err)
	}
	if _, err := (conformance.DownloadFileNameCase{Name: "both", Text: "a", UTF16: conformance.CodeUnits{"0061"}}).Input(); err == nil {
		t.Error("a case that gives both the text and its code units must be rejected")
	}
}
