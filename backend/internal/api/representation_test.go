package api

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestNegotiatedRepresentationsAndWritePreflight(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: respondError})
	writes := 0
	app.Post("/", func(c *fiber.Ctx) error {
		if err := representation(c, false); err != nil {
			return err
		}
		writes++
		return sendRepresentation(c, json.RawMessage(`{"id":9223372036854775807,"name":"Öğrenci & <test>","active":false,"missing":null,"rooms":[],"control":"\u0001"}`))
	})
	for _, tc := range []struct {
		accept, media string
		status        int
	}{
		{"", jsonMedia, 200}, {"*/*", jsonMedia, 200}, {"text/plain;q=1,application/json;q=.5", "text/plain", 200},
		{"application/xml", "application/xml", 200}, {"application/xml,*/*", "application/xml", 200},
		{"application/json;q=0,*/*;q=1", "application/xml", 200},
		{"application/json;q=NaN", jsonMedia, 406}, {"application/xml;charset=iso-8859-1", jsonMedia, 406},
		{"application/atom+xml", jsonMedia, 406}, {"text/html", jsonMedia, 406},
		{"application/json;q=0,application/xml;q=0,text/plain;q=0,*/*;q=1", jsonMedia, 406},
	} {
		before := writes
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Accept", tc.accept)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.status || !strings.HasPrefix(resp.Header.Get("Content-Type"), tc.media) || resp.Header.Get("Vary") != "Accept" {
			t.Fatalf("Accept %q: %d %v %s", tc.accept, resp.StatusCode, resp.Header, body)
		}
		if tc.status == 406 && writes != before {
			t.Fatal("negotiation failed after a write")
		}
		switch tc.media {
		case "application/xml":
			var document struct {
				Fields []struct {
					Name     string `xml:"name,attr"`
					Type     string `xml:"type,attr"`
					Encoding string `xml:"encoding,attr"`
					Text     string `xml:",chardata"`
				} `xml:"field"`
			}
			if err := xml.Unmarshal(body, &document); err != nil {
				t.Fatal(err)
			}
			found := map[string]string{}
			for _, field := range document.Fields {
				found[field.Name] = field.Type + ":" + field.Encoding + ":" + field.Text
			}
			if found["id"] != "number::9223372036854775807" || found["name"] != "string::Öğrenci & <test>" || found["control"] != "string:base64:AQ==" || found["rooms"] != "array::" || found["missing"] != "null::" || found["active"] != "boolean::false" {
				t.Fatalf("lost XML values: %v", found)
			}
		case "text/plain":
			var value map[string]json.RawMessage
			if json.Unmarshal(body, &value) != nil || string(value["id"]) != "9223372036854775807" {
				t.Fatalf("text lost fields: %s", body)
			}
		}
	}
}

func TestAtomCollectionHasMetadataAndLosslessEntryContent(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: respondError})
	app.Get("/records", func(c *fiber.Ctx) error {
		if err := representation(c, true); err != nil {
			return err
		}
		return sendRepresentation(c, json.RawMessage(`{"data":[{"id":1,"name":"A & <B>"}],"page":2,"page_size":1,"total":3}`))
	})
	req := httptest.NewRequest("GET", "/records?page=2&page_size=1", nil)
	req.Header.Set("Accept", "application/atom+xml")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	type entry struct {
		ID      string `xml:"id"`
		Title   string `xml:"title"`
		Updated string `xml:"updated"`
		Content string `xml:"content"`
	}
	var feed struct {
		XMLName xml.Name
		ID      string `xml:"id"`
		Title   string `xml:"title"`
		Updated string `xml:"updated"`
		Author  struct {
			Name string `xml:"name"`
		} `xml:"author"`
		Link struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Pagination struct {
			Page  int `xml:"page,attr"`
			Total int `xml:"total,attr"`
		} `xml:"urn:wad:pagination pagination"`
		Entries []entry `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || feed.XMLName.Space != "http://www.w3.org/2005/Atom" || feed.ID == "" || feed.Title == "" || feed.Updated == "" || feed.Author.Name == "" || feed.Link.Href != "/records?page=2&page_size=1" || feed.Pagination.Page != 2 || feed.Pagination.Total != 3 || len(feed.Entries) != 1 {
		t.Fatalf("invalid Atom: %s", body)
	}
	var value map[string]any
	row := feed.Entries[0]
	if row.ID == "" || row.Updated == "" || row.Title == "" {
		t.Fatalf("missing entry metadata: %s", body)
	}
	if err := json.Unmarshal([]byte(row.Content), &value); err != nil || value["name"] != "A & <B>" {
		t.Fatalf("lost Atom entry: %s", body)
	}
}
