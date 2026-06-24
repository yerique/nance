package cache

import (
	"go.mongodb.org/mongo-driver/bson"
)

// CachedResult is the on-wire-agnostic payload stored in Redis.
type CachedResult struct {
	NS   string     `bson:"ns"`
	Docs []bson.Raw `bson:"docs"`
	Cmd  string     `bson:"cmd"`
}

// Serialize packs docs for storage.
func Serialize(ns, cmd string, docs []bson.Raw) ([]byte, error) {
	cr := CachedResult{NS: ns, Docs: docs, Cmd: cmd}
	return bson.Marshal(cr)
}

// Deserialize unpacks a cached payload.
func Deserialize(b []byte) (*CachedResult, error) {
	var cr CachedResult
	if err := bson.Unmarshal(b, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

// ReplyFromCache builds a find/aggregate style cursor reply with id=0 (Phase 2 MVP).
func ReplyFromCache(cr *CachedResult) bson.D {
	return ReplyFromCacheWithCursor(cr, 0, nil)
}

// ReplyFromCacheWithCursor builds a reply; if cursorID!=0 only firstBatch docs are included.
func ReplyFromCacheWithCursor(cr *CachedResult, cursorID int64, firstBatchDocs []bson.Raw) bson.D {
	src := cr.Docs
	if firstBatchDocs != nil {
		src = firstBatchDocs
	}
	batch := make(bson.A, 0, len(src))
	for _, d := range src {
		var m bson.M
		if err := bson.Unmarshal(d, &m); err != nil {
			batch = append(batch, d)
			continue
		}
		batch = append(batch, m)
	}
	return bson.D{
		{Key: "cursor", Value: bson.D{
			{Key: "id", Value: cursorID},
			{Key: "ns", Value: cr.NS},
			{Key: "firstBatch", Value: batch},
		}},
		{Key: "ok", Value: float64(1)},
		{Key: "nanceCacheHit", Value: true},
	}
}

// DocsFromCursorReply extracts documents from a backend find/aggregate style reply.
func DocsFromCursorReply(reply any) (ns string, docs []bson.Raw, ok bool) {
	switch r := reply.(type) {
	case bson.D:
		return docsFromD(r)
	case bson.M:
		d := make(bson.D, 0, len(r))
		for k, v := range r {
			d = append(d, bson.E{Key: k, Value: v})
		}
		return docsFromD(d)
	default:
		return "", nil, false
	}
}

func docsFromD(reply bson.D) (string, []bson.Raw, bool) {
	var cursor any
	for _, e := range reply {
		if e.Key == "cursor" {
			cursor = e.Value
			break
		}
	}
	if cursor == nil {
		return "", nil, false
	}
	var ns string
	var batch bson.A
	switch c := cursor.(type) {
	case bson.D:
		for _, e := range c {
			switch e.Key {
			case "ns":
				if s, ok := e.Value.(string); ok {
					ns = s
				}
			case "firstBatch":
				if a, ok := e.Value.(bson.A); ok {
					batch = a
				}
			}
		}
	case bson.M:
		if s, ok := c["ns"].(string); ok {
			ns = s
		}
		if a, ok := c["firstBatch"].(bson.A); ok {
			batch = a
		}
	default:
		return "", nil, false
	}
	out := make([]bson.Raw, 0, len(batch))
	for _, item := range batch {
		raw, err := bson.Marshal(item)
		if err != nil {
			continue
		}
		out = append(out, raw)
	}
	return ns, out, true
}
