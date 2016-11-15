package jcfg

import (
	"io/ioutil"
	"testing"
)

type lexTest struct {
	name   string
	input  string
	tokens []token
}

var (
	tEOF          = token{tokenEOF, 0, ""}
	tESColon      = token{tokenEndStatement, 0, ";"}
	tESNewline    = token{tokenEndStatement, 0, "\n"}
	tESEmpty      = token{tokenEndStatement, 0, ""}
	tSectionStart = token{tokenSectionStart, 0, "{"}
	tSectionEnd   = token{tokenSectionEnd, 0, "}"}
)

var lexTests = []lexTest{
	{"empty", "", []token{tEOF}},
	{"bool keyword", "keyword;", []token{
		token{tokenKeyword, 0, "keyword"},
		tESColon,
		tEOF,
	}},
	{"bool keyword nocolon", "keyword", []token{
		token{tokenKeyword, 0, "keyword"},
		tESEmpty,
		tEOF,
	}},
	{"keyword 1 value", "keyword value1;", []token{
		token{tokenKeyword, 0, "keyword"},
		token{tokenValue, 0, "value1"},
		tESColon,
		tEOF,
	}},
	{"keyword 2 value", "keyword value1 value2;", []token{
		token{tokenKeyword, 0, "keyword"},
		token{tokenValue, 0, "value1"},
		token{tokenValue, 0, "value2"},
		tESColon,
		tEOF,
	}},
	{"block comment", "    /* Hello World */     ", []token{
		token{tokenBlockComment, 0, "/* Hello World */"},
		tEOF,
	}},
	{"line comment", "// Hello World", []token{
		token{tokenLineComment, 0, "// Hello World"},
		tEOF,
	}},
	{"keyword, value, line comment", "keyword1 value1; // Hello World", []token{
		token{tokenKeyword, 0, "keyword1"},
		token{tokenValue, 0, "value1"},
		tESColon,
		token{tokenLineComment, 0, "// Hello World"},
		tEOF,
	}},
	{"keyword, value, line comment nocolon", "keyword1 value1 // Hello World", []token{
		token{tokenKeyword, 0, "keyword1"},
		token{tokenValue, 0, "value1"},
		tESEmpty,
		token{tokenLineComment, 0, "// Hello World"},
		tEOF,
	}},
	{"hash comment", "# Hello World", []token{
		token{tokenHashComment, 0, "# Hello World"},
		tEOF,
	}},
	{"keyword, value, hash comment", "keyword1 value1; # Hello World", []token{
		token{tokenKeyword, 0, "keyword1"},
		token{tokenValue, 0, "value1"},
		tESColon,
		token{tokenHashComment, 0, "# Hello World"},
		tEOF,
	}},
	{"keyword, value, hash comment nocolon", "keyword1 value1 # Hello World", []token{
		token{tokenKeyword, 0, "keyword1"},
		token{tokenValue, 0, "value1"},
		tESEmpty,
		token{tokenHashComment, 0, "# Hello World"},
		tEOF,
	}},
	{"bool keyword eol", "keyword\n", []token{
		token{tokenKeyword, 0, "keyword"},
		tESNewline,
		tEOF,
	}},
	{"empty section", "section { }", []token{
		token{tokenKeyword, 0, "section"},
		tSectionStart,
		tSectionEnd,
		tEOF,
	}},
	{"section w/ value", "section { keyword1 value1; }", []token{
		token{tokenKeyword, 0, "section"},
		tSectionStart,
		token{tokenKeyword, 0, "keyword1"},
		token{tokenValue, 0, "value1"},
		tESColon,
		tSectionEnd,
		tEOF,
	}},
	{"modifier", "replace: keyword1 value1;", []token{
		token{tokenModifier, 0, "replace"},
		token{tokenKeyword, 0, "keyword1"},
		token{tokenValue, 0, "value1"},
		tESColon,
		tEOF,
	}},
}

func collect(t *lexTest) []token {
	tokens := []token{}
	l := lex(t.name, t.input)
	for {
		token := l.nextToken()
		tokens = append(tokens, token)
		if token.typ == tokenEOF || token.typ == tokenError {
			break
		}
	}
	return tokens
}

func equal(t1, t2 []token) bool {
	if len(t1) != len(t2) {
		return false
	}

	for i := range t1 {
		if t1[i].typ != t2[i].typ {
			return false
		}

		if t1[i].val != t2[i].val {
			return false
		}
	}
	return true
}

func TestLex(t *testing.T) {
	for _, test := range lexTests {
		t.Logf("Running test: %s", test.name)
		tokens := collect(&test)
		if !equal(tokens, test.tokens) {
			t.Errorf("input: '%s'\n%s: got\n\t%+v\nexpected\n\t%v", test.input, test.name, tokens, test.tokens)
		}
	}
}

type lexFileTest struct {
	filename string
	tokens   []token
}

var lexFileTests = []lexFileTest{
	{"testdata/junos-factory.config", []token{
		token{tokenKeyword, 0, "system"}, // system {
		tSectionStart,                    //
		token{tokenKeyword, 0, "syslog"}, //   syslog {
		tSectionStart,                    //
		token{tokenKeyword, 0, "file"},   //     file messages {
		token{tokenValue, 0, "messages"}, //
		tSectionStart,                    //       any notice;
		token{tokenKeyword, 0, "any"},    //
		token{tokenValue, 0, "notice"},   //
		tESColon,                         //
		token{tokenKeyword, 0, "authorization"},        //       authorization info;
		token{tokenValue, 0, "info"},                   //
		tESColon,                                       //
		tSectionEnd,                                    //    }
		token{tokenKeyword, 0, "file"},                 //     file interactive-commands {
		token{tokenValue, 0, "interactive-commands"},   //
		tSectionStart,                                  //
		token{tokenKeyword, 0, "interactive-commands"}, //       interactive-commands any;
		token{tokenValue, 0, "any"},                    //
		tESColon,                                       //
		tSectionEnd,                                    //     }
		token{tokenKeyword, 0, "user"},                 //     user "*" {
		token{tokenValue, 0, `"*"`},                    //
		tSectionStart,                                  //
		token{tokenKeyword, 0, "any"},                  //       any emergency;
		token{tokenValue, 0, "emergency"},              //
		tESColon,    //
		tSectionEnd, //   }
		tSectionEnd, //   }
		tSectionEnd, // }
		tEOF,
	}},
}

func TestFileLex(t *testing.T) {
	for _, filetest := range lexFileTests {
		input, err := ioutil.ReadFile(filetest.filename)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Running test on file: '%s'", filetest.filename)
		test := lexTest{filetest.filename, string(input), filetest.tokens}
		tokens := collect(&test)
		if !equal(tokens, test.tokens) {
			t.Errorf("input: '%s'\n%s: got\n\t%+v\nexpected\n\t%v", test.input, test.name, tokens, test.tokens)
		}
	}
}
