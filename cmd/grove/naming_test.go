package main

import "testing"

func TestModuleNamesPreserveGoInitialisms(t *testing.T) {
	for _, input := range []string{"HTTPClient", "HttpClient", "http_client", "http-client"} {
		spec, err := newModuleSpec("example.com/app", input, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if spec.Name != "HTTPClient" || spec.Snake != "http_client" || spec.Var != "httpClient" || spec.Table != "http_clients" || spec.RoutePath != "/http-clients" {
			t.Fatalf("%s: inconsistent names: %+v", input, spec)
		}
	}
	fields, err := parseFields("user_id:string,api_url:string")
	if err != nil {
		t.Fatal(err)
	}
	if fields[0].GoName != "UserID" || fields[1].GoName != "APIURL" {
		t.Fatalf("unexpected field names: %+v", fields)
	}
}

func TestRejectBuildConstrainedModuleFilenames(t *testing.T) {
	for _, name := range []string{"InvoiceTest", "InvoiceLinux", "InvoiceDarwin", "InvoiceAMD64", "InvoiceLinuxARM64", "_hidden"} {
		if _, err := newModuleSpec("example.com/app", name, "", nil); err == nil {
			t.Errorf("accepted hidden/build-constrained module %q", name)
		}
	}
}
