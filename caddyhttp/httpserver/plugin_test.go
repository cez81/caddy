package httpserver

import (
	"strings"
	"testing"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddyfile"
)

func TestStandardizeAddress(t *testing.T) {
	for i, test := range []struct {
		input                    string
		scheme, host, port, path string
		shouldErr                bool
	}{
		{`localhost`, "", "localhost", "", "", false},
		{`localhost:1234`, "", "localhost", "1234", "", false},
		{`localhost:`, "", "localhost", "", "", false},
		{`0.0.0.0`, "", "0.0.0.0", "", "", false},
		{`127.0.0.1:1234`, "", "127.0.0.1", "1234", "", false},
		{`:1234`, "", "", "1234", "", false},
		{`[::1]`, "", "::1", "", "", false},
		{`[::1]:1234`, "", "::1", "1234", "", false},
		{`:`, "", "", "", "", false},
		{`localhost:http`, "http", "localhost", "80", "", false},
		{`localhost:https`, "https", "localhost", "443", "", false},
		{`:http`, "http", "", "80", "", false},
		{`:https`, "https", "", "443", "", false},
		{`http://localhost:https`, "", "", "", "", true}, // conflict
		{`http://localhost:http`, "", "", "", "", true},  // repeated scheme
		{`http://localhost:443`, "", "", "", "", true},   // not conventional
		{`https://localhost:80`, "", "", "", "", true},   // not conventional
		{`http://localhost`, "http", "localhost", "80", "", false},
		{`https://localhost`, "https", "localhost", "443", "", false},
		{`http://127.0.0.1`, "http", "127.0.0.1", "80", "", false},
		{`https://127.0.0.1`, "https", "127.0.0.1", "443", "", false},
		{`http://[::1]`, "http", "::1", "80", "", false},
		{`http://localhost:1234`, "http", "localhost", "1234", "", false},
		{`https://127.0.0.1:1234`, "https", "127.0.0.1", "1234", "", false},
		{`http://[::1]:1234`, "http", "::1", "1234", "", false},
		{``, "", "", "", "", false},
		{`::1`, "", "::1", "", "", true},
		{`localhost::`, "", "localhost::", "", "", true},
		{`#$%@`, "", "", "", "", true},
		{`host/path`, "", "host", "", "/path", false},
		{`http://host/`, "http", "host", "80", "/", false},
		{`//asdf`, "", "asdf", "", "", false},
		{`:1234/asdf`, "", "", "1234", "/asdf", false},
		{`http://host/path`, "http", "host", "80", "/path", false},
		{`https://host:443/path/foo`, "https", "host", "443", "/path/foo", false},
		{`host:80/path`, "", "host", "80", "/path", false},
		{`host:https/path`, "https", "host", "443", "/path", false},
		{`/path`, "", "", "", "/path", false},
		{`http://idnhost-öåä`, "http", "xn--idnhost--7zaj0q", "80", "", false},
		{`https://idnhost-öåä`, "https", "xn--idnhost--7zaj0q", "443", "", false},
		{`idnhost-öåä`, "", "xn--idnhost--7zaj0q", "", "", false},
		{`idnhost-öåä:https/path`, "https", "xn--idnhost--7zaj0q", "443", "/path", false},
	} {
		actual, err := standardizeAddress(test.input)

		if err != nil && !test.shouldErr {
			t.Errorf("Test %d (%s): Expected no error, but had error: %v", i, test.input, err)
		}
		if err == nil && test.shouldErr {
			t.Errorf("Test %d (%s): Expected error, but had none", i, test.input)
		}

		if !test.shouldErr && actual.Original != test.input {
			t.Errorf("Test %d (%s): Expected original '%s', got '%s'", i, test.input, test.input, actual.Original)
		}
		if actual.Scheme != test.scheme {
			t.Errorf("Test %d (%s): Expected scheme '%s', got '%s'", i, test.input, test.scheme, actual.Scheme)
		}
		if actual.Host != test.host {
			t.Errorf("Test %d (%s): Expected host '%s', got '%s'", i, test.input, test.host, actual.Host)
		}
		if actual.Port != test.port {
			t.Errorf("Test %d (%s): Expected port '%s', got '%s'", i, test.input, test.port, actual.Port)
		}
		if actual.Path != test.path {
			t.Errorf("Test %d (%s): Expected path '%s', got '%s'", i, test.input, test.path, actual.Path)
		}
	}
}

func TestHostIsIDN(t *testing.T) {
	for i, test := range []struct {
		host     string
		expected bool
	}{
		{"idnhost-öåä", true},
		{"host", false},
		{"", false},
		{"1.2.3.4", false},
		{"::1", false},
		{"[::1]", false},
	} {
		actual := isIDN(test.host)
		if actual != test.expected {
			t.Errorf("Test %d: expected '%t' but got '%t'", i, test.expected, actual)
		}
	}
}

func TestAddressVHost(t *testing.T) {
	for i, test := range []struct {
		addr     Address
		expected string
	}{
		{Address{Original: "host:1234", Scheme: "", Host: "host", Port: "1234", Path: ""}, "host:1234"},
		{Address{Original: "host:1234/foo", Scheme: "", Host: "host", Port: "1234", Path: "/foo"}, "host:1234/foo"},
		{Address{Original: "host/foo", Scheme: "", Host: "host", Port: "", Path: "/foo"}, "host/foo"},
		{Address{Original: "http://host/foo", Scheme: "http", Host: "host", Port: "", Path: "/foo"}, "host/foo"},
		{Address{Original: "https://host/foo", Scheme: "https", Host: "host", Port: "", Path: "/foo"}, "host/foo"},
		{Address{Original: "idnhost-öåä:1234/foo", Scheme: "", Host: "xn--idnhost--7zaj0q", Port: "1234", Path: "/foo"}, "xn--idnhost--7zaj0q:1234/foo"},
		{Address{Original: "http://idnhost-öåä:1234/foo", Scheme: "http", Host: "xn--idnhost--7zaj0q", Port: "1234", Path: "/foo"}, "xn--idnhost--7zaj0q:1234/foo"},
	} {
		actual := test.addr.VHost()
		if actual != test.expected {
			t.Errorf("Test %d: expected '%s' but got '%s'", i, test.expected, actual)
		}
	}
}

func TestAddressString(t *testing.T) {
	for i, test := range []struct {
		addr     Address
		expected string
	}{
		{Address{Scheme: "http", Host: "host", Port: "1234", Path: "/path"}, "http://host:1234/path"},
		{Address{Scheme: "", Host: "host", Port: "", Path: ""}, "http://host"},
		{Address{Scheme: "", Host: "host", Port: "80", Path: ""}, "http://host"},
		{Address{Scheme: "", Host: "host", Port: "443", Path: ""}, "https://host"},
		{Address{Scheme: "https", Host: "host", Port: "443", Path: ""}, "https://host"},
		{Address{Scheme: "https", Host: "host", Port: "", Path: ""}, "https://host"},
		{Address{Scheme: "", Host: "host", Port: "80", Path: "/path"}, "http://host/path"},
		{Address{Scheme: "http", Host: "", Port: "1234", Path: ""}, "http://:1234"},
		{Address{Scheme: "", Host: "", Port: "", Path: ""}, ""},
	} {
		actual := test.addr.String()
		if actual != test.expected {
			t.Errorf("Test %d: expected '%s' but got '%s'", i, test.expected, actual)
		}
	}
}

func TestInspectServerBlocksWithCustomDefaultPort(t *testing.T) {
	Port = "9999"
	filename := "Testfile"
	ctx := newContext().(*httpContext)
	input := strings.NewReader(`localhost`)
	sblocks, err := caddyfile.Parse(filename, input, nil)
	if err != nil {
		t.Fatalf("Expected no error setting up test, got: %v", err)
	}
	_, err = ctx.InspectServerBlocks(filename, sblocks)
	if err != nil {
		t.Fatalf("Didn't expect an error, but got: %v", err)
	}
	addr := ctx.keysToSiteConfigs["localhost"].Addr
	if addr.Port != Port {
		t.Errorf("Expected the port on the address to be set, but got: %#v", addr)
	}
}

func TestInspectServerBlocksCaseInsensitiveKey(t *testing.T) {
	filename := "Testfile"
	ctx := newContext().(*httpContext)
	input := strings.NewReader("localhost {\n}\nLOCALHOST {\n}")
	sblocks, err := caddyfile.Parse(filename, input, nil)
	if err != nil {
		t.Fatalf("Expected no error setting up test, got: %v", err)
	}
	_, err = ctx.InspectServerBlocks(filename, sblocks)
	if err == nil {
		t.Error("Expected an error because keys on this server type are case-insensitive (so these are duplicated), but didn't get an error")
	}
}

func TestGetConfig(t *testing.T) {
	// case insensitivity for key
	con := caddy.NewTestController("http", "")
	con.Key = "foo"
	cfg := GetConfig(con)
	con.Key = "FOO"
	cfg2 := GetConfig(con)
	if cfg != cfg2 {
		t.Errorf("Expected same config using same key with different case; got %p and %p", cfg, cfg2)
	}

	// make sure different key returns different config
	con.Key = "foobar"
	cfg3 := GetConfig(con)
	if cfg == cfg3 {
		t.Errorf("Expected different configs using when key is different; got %p and %p", cfg, cfg3)
	}
}

func TestDirectivesList(t *testing.T) {
	for i, dir1 := range directives {
		if dir1 == "" {
			t.Errorf("directives[%d]: empty directive name", i)
			continue
		}
		if got, want := dir1, strings.ToLower(dir1); got != want {
			t.Errorf("directives[%d]: %s should be lower-cased", i, dir1)
			continue
		}
		for j := i + 1; j < len(directives); j++ {
			dir2 := directives[j]
			if dir1 == dir2 {
				t.Errorf("directives[%d] (%s) is a duplicate of directives[%d] (%s)",
					j, dir2, i, dir1)
			}
		}
	}
}

func TestContextSaveConfig(t *testing.T) {
	ctx := newContext().(*httpContext)
	ctx.saveConfig("foo", new(SiteConfig))
	if _, ok := ctx.keysToSiteConfigs["foo"]; !ok {
		t.Error("Expected config to be saved, but it wasn't")
	}
	if got, want := len(ctx.siteConfigs), 1; got != want {
		t.Errorf("Expected len(siteConfigs) == %d, but was %d", want, got)
	}
	ctx.saveConfig("Foobar", new(SiteConfig))
	if _, ok := ctx.keysToSiteConfigs["foobar"]; ok {
		t.Error("Did not expect to get config with case-insensitive key, but did")
	}
	if got, want := len(ctx.siteConfigs), 2; got != want {
		t.Errorf("Expected len(siteConfigs) == %d, but was %d", want, got)
	}
}

// Test to make sure we are correctly hiding the Caddyfile
func TestHideCaddyfile(t *testing.T) {
	ctx := newContext().(*httpContext)
	ctx.saveConfig("test", &SiteConfig{
		Root:            Root,
		originCaddyfile: "Testfile",
	})
	err := hideCaddyfile(ctx)
	if err != nil {
		t.Fatalf("Failed to hide Caddyfile, got: %v", err)
		return
	}
	if len(ctx.siteConfigs[0].HiddenFiles) == 0 {
		t.Fatal("Failed to add Caddyfile to HiddenFiles.")
		return
	}
	for _, file := range ctx.siteConfigs[0].HiddenFiles {
		if file == "/Testfile" {
			return
		}
	}
	t.Fatal("Caddyfile missing from HiddenFiles")
}
