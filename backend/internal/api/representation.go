package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const jsonMedia = "application/json"

func representation(c *fiber.Ctx, collection bool) error {
	supported := []string{jsonMedia, "application/xml", "text/plain"}
	if collection {
		supported = append(supported, "application/atom+xml")
	}
	return chooseRepresentation(c, supported)
}

// A specific exclusion overrides a wildcard; quality then specificity rank the remaining formats.
func chooseRepresentation(c *fiber.Ctx, supported []string) error {
	c.Vary("Accept")
	header := c.Get("Accept")
	if header == "" {
		header = "*/*"
	}
	best, bestQuality, bestSpecificity := "", 0.0, -1
	for _, candidate := range supported {
		quality, specificity := 0.0, -1
		for _, entry := range strings.Split(header, ",") {
			media, params, err := mime.ParseMediaType(strings.TrimSpace(entry))
			if err != nil {
				continue
			}
			level := -1
			if media == "*/*" {
				level = 0
			} else if media == strings.SplitN(candidate, "/", 2)[0]+"/*" {
				level = 1
			} else if media == candidate {
				level = 2
			}
			if level < 0 {
				continue
			}
			q := 1.0
			if value, ok := params["q"]; ok {
				q, err = strconv.ParseFloat(value, 64)
			}
			if err != nil || !(q >= 0 && q <= 1) {
				continue
			}
			matches := true
			for key, value := range params {
				if key != "q" && (key != "charset" || !strings.EqualFold(value, "utf-8")) {
					matches = false
				}
			}
			if !matches {
				continue
			}
			if level > specificity || level == specificity && q > quality {
				quality, specificity = q, level
			}
		}
		if quality > 0 && (quality > bestQuality || quality == bestQuality && specificity > bestSpecificity) {
			best, bestQuality, bestSpecificity = candidate, quality, specificity
		}
	}
	if best == "" {
		return fiber.NewError(406, "Choose a supported response media type for this endpoint.")
	}
	c.Locals("representation", best)
	return nil
}

func sendRepresentation(c *fiber.Ctx, body json.RawMessage) error {
	media, _ := c.Locals("representation").(string)
	if media == "" {
		media = jsonMedia
	}
	c.Set("Content-Type", media+"; charset=utf-8")
	switch media {
	case jsonMedia:
		return c.Send(body)
	case "text/plain":
		var output bytes.Buffer
		if err := json.Indent(&output, body, "", "  "); err != nil {
			return err
		}
		return c.Send(output.Bytes())
	case "application/xml":
		var value any
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		var output bytes.Buffer
		output.WriteString(xml.Header)
		encoder := xml.NewEncoder(&output)
		if err := xmlValue(encoder, xml.StartElement{Name: xml.Name{Local: "response"}}, value); err != nil {
			return err
		}
		if err := encoder.Flush(); err != nil {
			return err
		}
		return c.Send(output.Bytes())
	case "application/atom+xml":
		return sendAtom(c, body)
	}
	return fmt.Errorf("unknown internal representation")
}

func sendValue(c *fiber.Ctx, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return sendRepresentation(c, body)
}

// Field names stay attributes, and explicit types preserve nulls, empty arrays, and large IDs.
func xmlValue(encoder *xml.Encoder, start xml.StartElement, value any) error {
	kind, content := "null", ""
	switch v := value.(type) {
	case map[string]any:
		kind = "object"
	case []any:
		kind = "array"
	case string:
		kind, content = "string", v
		for _, r := range v {
			if !(r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF) {
				content = base64.StdEncoding.EncodeToString([]byte(v))
				start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "encoding"}, Value: "base64"})
				break
			}
		}
	case bool:
		kind, content = "boolean", strconv.FormatBool(v)
	case json.Number:
		kind, content = "number", v.String()
	}
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "type"}, Value: kind})
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			field := xml.StartElement{Name: xml.Name{Local: "field"}, Attr: []xml.Attr{{Name: xml.Name{Local: "name"}, Value: key}}}
			if err := xmlValue(encoder, field, v[key]); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := xmlValue(encoder, xml.StartElement{Name: xml.Name{Local: "item"}}, item); err != nil {
				return err
			}
		}
	default:
		if err := encoder.EncodeToken(xml.CharData(content)); err != nil {
			return err
		}
	}
	return encoder.EncodeToken(start.End())
}

type atomEntry struct {
	ID      string `xml:"id"`
	Title   string `xml:"title"`
	Updated string `xml:"updated"`
	Content struct {
		Type string `xml:"type,attr"`
		Text string `xml:",chardata"`
	} `xml:"content"`
}

func sendAtom(c *fiber.Ctx, body json.RawMessage) error {
	var page struct {
		Data     []json.RawMessage `json:"data"`
		Page     int               `json:"page"`
		PageSize int               `json:"page_size"`
		Total    int64             `json:"total"`
	}
	if err := json.Unmarshal(body, &page); err != nil || page.Data == nil {
		return fmt.Errorf("invalid internal Atom collection")
	}
	feed := struct {
		XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
		ID      string   `xml:"id"`
		Title   string   `xml:"title"`
		Updated string   `xml:"updated"`
		Author  struct {
			Name string `xml:"name"`
		} `xml:"author"`
		Link struct {
			Rel  string `xml:"rel,attr"`
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Pagination struct {
			Page     int   `xml:"page,attr"`
			PageSize int   `xml:"page_size,attr"`
			Total    int64 `xml:"total,attr"`
		} `xml:"urn:wad:pagination pagination"`
		Entries []atomEntry `xml:"entry"`
	}{ID: fmt.Sprintf("urn:wad:feed:%x", sha256.Sum256([]byte(c.OriginalURL()))), Title: "Faculty Exams · " + c.Path(), Updated: time.Now().UTC().Format(time.RFC3339)}
	feed.Author.Name = "Faculty Exams"
	feed.Link.Rel, feed.Link.Href = "self", c.OriginalURL()
	feed.Pagination.Page, feed.Pagination.PageSize, feed.Pagination.Total = page.Page, page.PageSize, page.Total
	for _, row := range page.Data {
		entry := atomEntry{ID: fmt.Sprintf("urn:wad:entry:%x", sha256.Sum256(append([]byte(c.Path()), row...))), Title: "Resource", Updated: feed.Updated}
		entry.Content.Type, entry.Content.Text = jsonMedia, string(row)
		feed.Entries = append(feed.Entries, entry)
	}
	output, err := xml.Marshal(feed)
	if err != nil {
		return err
	}
	return c.Send(append([]byte(xml.Header), output...))
}
