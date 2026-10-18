# Response representations

Ordinary resources and workflow responses support `application/json`, `application/xml`, and `text/plain`. Collection reads also support `application/atom+xml`. These formats contain the same authorized, filtered data and page metadata. Request bodies remain UTF-8 JSON.

Send `Accept` to select a format. Absent headers or `*/*` select JSON. Quality values rank formats; more specific ranges override wildcard quality, including explicit `q=0` exclusions. Ties prefer the more specific range and then JSON, XML, text, Atom in that order. An unsupported format returns 406 before a write. Responses include `Vary: Accept` and a UTF-8 content type. Empty 204 deletes do not negotiate a response body.

Errors always use the documented JSON error envelope, including on 406 and on failures of XML/text/Atom requests. Health endpoints and browser authentication configuration remain JSON-only; Keycloak owns its protocol endpoints. XLSX and database archives use their own binary media types.

## XML

The root is `response`. Object fields are `field` elements with a `name` attribute. Arrays contain `item` elements. Each element has a `type` attribute: `object`, `array`, `string`, `number`, `boolean`, or `null`. Object field order is immaterial; array order is preserved. Numbers retain their exact decimal text. Empty arrays, empty objects, nulls, and empty strings stay distinguishable.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<response type="object">
  <field name="id" type="number">7</field>
  <field name="name" type="string">Öğrenci &amp; Faculty</field>
  <field name="rooms" type="array"><item type="object"><field name="name" type="string">Hall</field></item></field>
  <field name="missing" type="null"></field>
</response>
```

XML markup is escaped. A string containing characters forbidden by XML 1.0 has `encoding="base64"`; decode its UTF-8 bytes from Base64 to recover the original value. This preserves text without silently replacing characters.

## Plain text

`text/plain` contains an indented JSON document. It retains every field, nested value, null, and exact number while remaining readable in a browser or terminal.

## Atom

Collections use the Atom namespace `http://www.w3.org/2005/Atom`, with feed ID, title, generation timestamp in UTC, author, self link, and one entry per row on the current page. Each entry has an absolute URN ID, title, timestamp, and `content type="application/json"` containing the complete row as escaped JSON text. IDs are deterministic hashes of the feed request or the resource path plus row content. They change when those inputs change; this is a collection representation, not a persistent change feed.

The `pagination` extension in namespace `urn:wad:pagination` carries `page`, `page_size`, and `total` attributes, including for empty/out-of-range pages. To read more results, repeat the same request with the next page. Atom is not offered for individual records, dashboard/own-workload summaries, creates, updates, or bulk action summaries.
