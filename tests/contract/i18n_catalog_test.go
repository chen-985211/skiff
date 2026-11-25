package contract_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	native "github.com/egoist/mygo/i18n"

	"skiff/internal/contexts/transfer/presentation/i18n"
)

var messageKey = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*(\.[a-z][a-zA-Z0-9]*)+$`)

func TestI18nCatalogsMatchChinese(t *testing.T) {
	root := i18nRepositoryRoot()
	dir := filepath.Join(root, "internal", "contexts", "transfer", "presentation", "i18n", "catalogs")
	base := readMessages(t, filepath.Join(dir, i18n.Source+".json"))
	if len(base) == 0 || len(i18n.Languages()) < 2 {
		t.Fatal("expected a Chinese source catalog and at least one translation")
	}
	for _, locale := range i18n.Languages() {
		t.Run(locale, func(t *testing.T) {
			translations := readMessages(t, filepath.Join(dir, locale+".json"))
			for key, args := range base {
				got, ok := translations[key]
				if !ok {
					t.Errorf("%s is missing %s", locale, key)
				} else if !reflect.DeepEqual(got, args) {
					t.Errorf("%s %s uses parameters %v, want %v", locale, key, got, args)
				}
			}
			for key := range translations {
				if _, ok := base[key]; !ok {
					t.Errorf("%s contains unknown key %s", locale, key)
				}
			}
		})
	}
}

func i18nRepositoryRoot() string {
	_, source, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(source), "../..")
}

func readMessages(t *testing.T, path string) map[string]map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	if first, err := decoder.Token(); err != nil || first != json.Delim('{') {
		t.Fatalf("%s: expected a JSON object, got %v (%v)", path, first, err)
	}
	result := map[string]map[string]string{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		key, ok := token.(string)
		if !ok || !messageKey.MatchString(key) {
			t.Fatalf("%s: invalid message key %v", path, token)
		}
		if _, duplicate := result[key]; duplicate {
			t.Fatalf("%s: duplicate message key %s", path, key)
		}
		var body string
		if err := decoder.Decode(&body); err != nil || strings.TrimSpace(body) == "" {
			t.Fatalf("%s: %s must be a nonempty message (%v)", path, key, err)
		}
		message, err := native.ParseMessage(body)
		if err != nil {
			t.Fatalf("%s: %s: %v", path, key, err)
		}
		args := map[string]string{}
		for _, arg := range message.Args() {
			args[arg.Name] = arg.Kind
		}
		result[key] = args
	}
	if last, err := decoder.Token(); err != nil || last != json.Delim('}') {
		t.Fatalf("%s: expected end of object, got %v (%v)", path, last, err)
	}
	if extra, err := decoder.Token(); err != io.EOF {
		t.Fatal(fmt.Errorf("%s: unexpected token after object: %v (%v)", path, extra, err))
	}
	return result
}
