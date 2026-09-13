//go:build linux

package platform

import "testing"

// Pure DragOffer shapes (no display needed).
func TestDragOfferMIMETypes(t *testing.T) {
	if got := (DragOffer{}).MIMETypes(); len(got) != 0 {
		t.Fatalf("empty offer mimes = %v", got)
	}
	got := DragOffer{
		Files: []string{"/tmp/a"},
		Data:  map[string][]byte{"text/plain": {1}, "image/png": {2}, "": {3}},
	}.MIMETypes()
	want := []string{mimeURIList, "image/png", "text/plain"}
	if len(got) != len(want) {
		t.Fatalf("mimes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mimes = %v, want %v", got, want)
		}
	}
	// Empty payloads are not servable, so they stay unannounced.
	empty := DragOffer{
		Data: map[string][]byte{"text/plain": {}, "image/png": {1}},
	}.MIMETypes()
	if len(empty) != 1 || empty[0] != "image/png" {
		t.Fatalf("mimes with empty value = %v, want [image/png]", empty)
	}
}

func TestDragOfferURIList(t *testing.T) {
	if s := (DragOffer{}).URIList(); s != "" {
		t.Fatalf("empty uri-list = %q", s)
	}
	s := DragOffer{Files: []string{"/tmp/a", "/tmp/sp ace"}}.URIList()
	if len(s) == 0 {
		t.Fatal("uri-list must not be empty")
	}
	files := parseURIList(s)
	if len(files) != 2 || files[0] != "/tmp/a" || files[1] != "/tmp/sp ace" {
		t.Fatalf("round-trip files = %v (raw %q)", files, s)
	}
}

func TestDragOfferPayload(t *testing.T) {
	if p := (DragOffer{}).OfferPayload(); p != nil {
		t.Fatalf("empty payload = %v", p)
	}
	o := DragOffer{
		Files: []string{"/tmp/a"},
		Data:  map[string][]byte{"text/plain": []byte("hi")},
	}
	p := o.OfferPayload()
	if len(p) != 2 || len(p[mimeURIList]) == 0 || string(p["text/plain"]) != "hi" {
		t.Fatalf("payload = %v", p)
	}
	// Explicit uri-list bytes win over Files; results are copies.
	o2 := DragOffer{
		Files: []string{"/tmp/a"},
		Data:  map[string][]byte{mimeURIList: []byte("file:///tmp/b\n")},
	}
	p2 := o2.OfferPayload()
	if string(p2[mimeURIList]) != "file:///tmp/b\n" {
		t.Fatalf("override payload = %q", p2[mimeURIList])
	}
	p2["text/plain"] = []byte("mut")
	if _, ok := o2.Data["text/plain"]; ok {
		t.Fatal("payload must not alias the offer map")
	}
}
